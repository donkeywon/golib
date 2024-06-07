package pipeline

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/ftp"
	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/bufferpool"
	sshutil "github.com/donkeywon/golib/util/ssh"
	"golang.org/x/crypto/ssh"
)

func init() {
	plugin.Register(RWTypeStore, func() interface{} { return NewStoreRW() })
	plugin.RegisterCfg(RWTypeStore, func() interface{} { return NewStoreCfg() })
}

type StoreType string

const (
	RWTypeStore RWType = "store"

	StoreTypeOss = "oss"
	StoreTypeFtp = "ftp"
	StoreTypeSSH = "ssh"
)

type OssCfg struct {
	Ak     string `json:"ak"     validate:"min=1" yaml:"ak"`
	Sk     string `json:"sk"     validate:"min=1" yaml:"sk"`
	Region string `json:"region" yaml:"region"`
	Append bool   `json:"append" yaml:"append"`
}

type FtpCfg struct {
	Addr string `json:"addr" validate:"min=1" yaml:"addr"`
	User string `json:"user" validate:"min=1" yaml:"user"`
	Pwd  string `json:"pwd"  validate:"min=1" yaml:"pwd"`
}

type SSHCfg struct {
	Addr       string `json:"addr"       validate:"min=1"  yaml:"addr"`
	User       string `json:"user"       validate:"min=1"  yaml:"user"`
	Pwd        string `json:"pwd"        yaml:"pwd"`
	PrivateKey string `json:"privateKey" yaml:"privateKey"`
}

type StoreRWCfg struct {
	Type     StoreType   `json:"type"     validate:"required" yaml:"type"`
	Cfg      interface{} `json:"cfg"      validate:"required" yaml:"cfg"`
	URL      string      `json:"url"      validate:"min=1"    yaml:"url"`
	Checksum string      `json:"checksum" yaml:"checksum"`
	Retry    int         `json:"retry"    validate:"gte=1"    yaml:"retry"`
	Timeout  int         `json:"timeout"  validate:"gte=1"    yaml:"timeout"`
}

func NewStoreCfg() *StoreRWCfg {
	return &StoreRWCfg{}
}

type StoreRW struct {
	RW
	*StoreRWCfg

	r io.ReadCloser
	w io.WriteCloser

	sshCmd       string
	sshCli       *ssh.Client
	sshSess      *ssh.Session
	sshStderrBuf *bufferpool.Buffer
}

func NewStoreRW() *StoreRW {
	return &StoreRW{
		RW: NewBaseRW(string(RWTypeStore)),
	}
}

func (s *StoreRW) Init() error {
	if s.Reader() != nil && s.Writer() != nil {
		return errs.New("store rw cannot has nested reader and writer at the same time")
	}

	var err error

	switch s.StoreRWCfg.Type {
	case StoreTypeFtp:
		err = s.initFtp()
	case StoreTypeOss:
		err = s.initOSS()
	case StoreTypeSSH:
		err = s.initSSH()
	default:
		return errs.Errorf("unknown store type: %+v", s.StoreRWCfg.Type)
	}

	if err != nil {
		return err
	}

	if s.IsReader() {
		_ = s.NestReader(s.r)
		s.EnableChecksum(s.StoreRWCfg.Checksum)
	} else if s.IsWriter() {
		_ = s.NestWriter(s.w)
	}
	s.EnableCalcHash()

	s.WithLoggerNoName(s.Logger(), "store", s.StoreRWCfg.Type)
	s.RegisterReadHook(s.hookLogRead)
	s.RegisterWriteHook(s.hookLogWrite)

	return s.RW.Init()
}

func (s *StoreRW) Start() error {
	defer func() {
		err := recover()
		if err != nil {
			s.AppendError(errs.Errorf("store panic: %+v", err))
		}

		closeErr := s.Close()
		if closeErr != nil {
			s.AppendError(errs.Wrap(closeErr, "store RW close fail"))
		}
	}()

	if s.StoreRWCfg.Type != StoreTypeSSH {
		return errs.Errorf("non-runnable store type: %+v", s.StoreRWCfg.Type)
	}
	err := s.sshSess.Run(s.sshCmd)
	if errors.Is(err, ErrStoppedManually) {
		err = nil
	}
	return err
}

func (s *StoreRW) Close() error {
	if s.StoreRWCfg.Type == StoreTypeSSH {
		closeErr := s.closeSSHSessionAndCli()
		if errors.Is(closeErr, io.EOF) {
			closeErr = nil
		}
		return errors.Join(closeErr, s.RW.Close())
	}
	return s.RW.Close()
}

func (s *StoreRW) Type() interface{} {
	return RWTypeStore
}

func (s *StoreRW) GetCfg() interface{} {
	return s.StoreRWCfg
}

func (s *StoreRW) initFtp() error {
	var err error
	cfg, _ := s.StoreRWCfg.Cfg.(*FtpCfg)
	if s.IsReader() {
		s.r, err = createFtpReader(s.StoreRWCfg, cfg)
	} else {
		s.w, err = createFtpWriter(s.StoreRWCfg, cfg)
	}
	return err
}

func (s *StoreRW) initOSS() error {
	var err error
	cfg, _ := s.StoreRWCfg.Cfg.(*OssCfg)
	if s.IsReader() {
		s.r, err = createOssReader(s.StoreRWCfg, cfg)
	} else {
		if cfg.Append {
			s.w, err = createOssAppendWriter(s.StoreRWCfg, cfg)
		} else {
			s.w, err = createOssMultipartWriter(s.StoreRWCfg, cfg)
		}
	}
	return err
}

func (s *StoreRW) initSSH() error {
	var err error
	cfg, _ := s.StoreRWCfg.Cfg.(*SSHCfg)
	if !s.IsStarter() {
		return errs.Errorf("ssh store must be Starter")
	}
	s.sshCli, s.sshSess, err = createSSHStarter(s.StoreRWCfg, cfg)
	if err != nil {
		return errs.Wrap(err, "create ssh cli and sess fail")
	}
	if s.Reader() != nil {
		s.sshCmd = sshWriteCmd(s.StoreRWCfg.URL)
		s.sshSess.Stdin = s
	} else if s.Writer() != nil {
		s.sshCmd = sshReadCmd(s.StoreRWCfg.URL)
		s.sshSess.Stdout = s
	} else {
		return errs.Errorf("ssh store must has Reader or Writer")
	}
	s.sshStderrBuf = bufferpool.GetBuffer()
	s.sshSess.Stderr = s.sshStderrBuf
	return nil
}

func (s *StoreRW) closeSSHSessionAndCli() error {
	// sshSess.Signal and sshSess.Close may return io.EOF
	return errors.Join(s.sshSess.Signal(ssh.SIGTERM), s.sshSess.Close(), s.sshCli.Close())
}

func (s *StoreRW) hookLogWrite(n int, bs []byte, err error, cost int64, misc ...interface{}) error {
	s.Info("write", "bs_len", len(bs), "bs_cap", cap(bs), "nw", n, "cost", cost,
		"async_chan_len", s.AsyncChanLen(), "async_chan_cap", s.AsyncChanCap(), "misc", misc, "err", err)
	return nil
}

func (s *StoreRW) hookLogRead(n int, bs []byte, err error, cost int64, misc ...interface{}) error {
	s.Info("read", "bs_len", len(bs), "bs_cap", cap(bs), "nr", n, "cost", cost,
		"async_chan_len", s.AsyncChanLen(), "async_chan_cap", s.AsyncChanCap(), "misc", misc, "err", err)
	return nil
}

func createFtpCfg(storeCfg *StoreRWCfg, ftpCfg *FtpCfg) *ftp.Cfg {
	return &ftp.Cfg{
		Addr:    ftpCfg.Addr,
		User:    ftpCfg.User,
		Pwd:     ftpCfg.Pwd,
		Timeout: storeCfg.Timeout,
		Retry:   storeCfg.Retry,
	}
}

func createFtpReader(storeCfg *StoreRWCfg, ftpCfg *FtpCfg) (*ftp.Reader, error) {
	r := ftp.NewReader()
	r.Cfg = createFtpCfg(storeCfg, ftpCfg)
	r.Path = storeCfg.URL

	err := r.Init()
	if err != nil {
		return nil, errs.Wrap(err, "init ftp reader fail")
	}

	return r, nil
}

func createFtpWriter(storeCfg *StoreRWCfg, ftpCfg *FtpCfg) (*ftp.Writer, error) {
	w := ftp.NewWriter()
	w.Cfg = createFtpCfg(storeCfg, ftpCfg)
	w.Path = storeCfg.URL

	err := w.Init()
	if err != nil {
		return nil, errs.Wrap(err, "init ftp writer fail")
	}

	return w, nil
}

func createOssCfg(storeCfg *StoreRWCfg, ossCfg *OssCfg) *oss.Cfg {
	return &oss.Cfg{
		URL:     storeCfg.URL,
		Ak:      ossCfg.Ak,
		Sk:      ossCfg.Sk,
		Retry:   storeCfg.Retry,
		Timeout: storeCfg.Timeout,
		Region:  ossCfg.Region,
	}
}

func createOssReader(storeCfg *StoreRWCfg, ossCfg *OssCfg) (*oss.Reader, error) {
	r := oss.NewReader()
	r.Cfg = createOssCfg(storeCfg, ossCfg)

	return r, nil
}

func createOssAppendWriter(storeCfg *StoreRWCfg, ossCfg *OssCfg) (*oss.AppendWriter, error) {
	w := oss.NewAppendWriter()
	w.Cfg = createOssCfg(storeCfg, ossCfg)
	return w, nil
}

func createOssMultipartWriter(storeCfg *StoreRWCfg, ossCfg *OssCfg) (*oss.MultiPartWriter, error) {
	w := oss.NewMultiPartWriter()
	w.Cfg = createOssCfg(storeCfg, ossCfg)
	return w, nil
}

func createSSHStarter(storeCfg *StoreRWCfg, sshCfg *SSHCfg) (*ssh.Client, *ssh.Session, error) {
	return sshutil.NewClient(sshCfg.Addr, sshCfg.User, sshCfg.Pwd, []byte(sshCfg.PrivateKey), storeCfg.Timeout)
}

func sshReadCmd(path string) string {
	return "cat " + path
}

func sshWriteCmd(path string) string {
	dir := filepath.Dir(path)
	return fmt.Sprintf("rm -f %s; mkdir -p %s; cat > %s", path, dir, path)
}

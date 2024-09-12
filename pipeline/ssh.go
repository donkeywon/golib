package pipeline

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/bufferpool"
	"github.com/donkeywon/golib/util/sshs"
	"golang.org/x/crypto/ssh"
)

func init() {
	plugin.RegisterWithCfg(RWTypeSSH, func() interface{} { return NewSSHRW() }, func() interface{} { return NewSSHRWCfg() })
}

const (
	RWTypeSSH RWType = "ssh"

	defaultSSHTimeout = 30
)

type SSHRWCfg struct {
	Addr       string `json:"addr"       validate:"required"  yaml:"addr"`
	User       string `json:"user"       validate:"required"  yaml:"user"`
	Pwd        string `json:"pwd"        yaml:"pwd"`
	PrivateKey string `json:"privateKey" yaml:"privateKey"`
	Timeout    int    `json:"timeout"    yaml:"timeout"`
	Path       string `json:"path"       validate:"required"  yaml:"path"`
}

func NewSSHRWCfg() *SSHRWCfg {
	return &SSHRWCfg{
		Timeout: defaultSSHTimeout,
	}
}

type SSHRW struct {
	RW
	*SSHRWCfg

	sshCmd       string
	sshCli       *ssh.Client
	sshSess      *ssh.Session
	sshStderrBuf *bufferpool.Buffer
}

func NewSSHRW() RW {
	return &SSHRW{
		RW: CreateBaseRW(string(RWTypeSSH)),
	}
}

func (s *SSHRW) Init() error {
	if !s.IsStarter() {
		return errs.New("ssh rw must be Starter")
	}

	var err error
	s.sshCli, s.sshSess, err = sshs.NewClient(s.SSHRWCfg.Addr, s.SSHRWCfg.User, s.SSHRWCfg.Pwd, []byte(s.SSHRWCfg.PrivateKey), s.SSHRWCfg.Timeout)
	if err != nil {
		return errs.Wrap(err, "create ssh cli and session fail")
	}
	if s.Reader() != nil {
		s.sshCmd = sshWriteCmd(s.SSHRWCfg.Path)
		s.sshSess.Stdin = s
	} else if s.Writer() != nil {
		s.sshCmd = sshReadCmd(s.SSHRWCfg.Path)
		s.sshSess.Stdout = s
	} else {
		return errs.Errorf("ssh rw must has Reader or Writer")
	}
	s.sshStderrBuf = bufferpool.GetBuffer()
	s.sshSess.Stderr = s.sshStderrBuf

	s.HookRead(s.hookLogRead)
	s.HookWrite(s.hookLogWrite)

	return s.RW.Init()
}

func (s *SSHRW) Start() error {
	err := s.sshSess.Run(s.sshCmd)
	if errors.Is(err, ErrStoppedManually) {
		err = nil
	}

	closeErr := sshs.Close(s.sshCli, s.sshSess)
	if closeErr != nil {
		closeErr = errs.Wrap(closeErr, "close ssh cli and session fail")
	}

	if err != nil {
		return errs.Wrapf(err, "ssh session run fail, cmd: %s", s.sshCmd)
	}
	return nil
}

func (s *SSHRW) Type() interface{} {
	return RWTypeSSH
}

func (s *SSHRW) GetCfg() interface{} {
	return s.SSHRWCfg
}

func (s *SSHRW) hookLogWrite(n int, bs []byte, err error, cost int64, misc ...interface{}) error {
	s.Info("write", "bs_len", len(bs), "bs_cap", cap(bs), "nw", n, "cost", cost,
		"async_chan_len", s.AsyncChanLen(), "async_chan_cap", s.AsyncChanCap(), "misc", misc, "err", err)
	return nil
}

func (s *SSHRW) hookLogRead(n int, bs []byte, err error, cost int64, misc ...interface{}) error {
	s.Info("read", "bs_len", len(bs), "bs_cap", cap(bs), "nr", n, "cost", cost,
		"async_chan_len", s.AsyncChanLen(), "async_chan_cap", s.AsyncChanCap(), "misc", misc, "err", err)
	return nil
}

func sshReadCmd(path string) string {
	return "cat " + path
}

func sshWriteCmd(path string) string {
	dir := filepath.Dir(path)
	return fmt.Sprintf("rm -f %s; mkdir -p %s; cat > %s", path, dir, path)
}

package rw

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/ftp"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(TypeFtp, func() RW { return NewFtp() }, func() any { return NewFtpCfg() })
}

const (
	TypeFtp Type = "ftp"

	defaultFtpTimeout = 600
	defaultFtpRetry   = 3
)

type FtpCfg struct {
	Addr    string `json:"addr"    validate:"required" yaml:"addr"`
	User    string `json:"user"    validate:"required" yaml:"user"`
	Pwd     string `json:"pwd"     validate:"required" yaml:"pwd"`
	Path    string `json:"path"    validate:"required" yaml:"path"`
	Timeout int    `json:"timeout" yaml:"timeout"`
	Retry   int    `json:"retry"   yaml:"retry"`
}

func NewFtpCfg() *FtpCfg {
	return &FtpCfg{
		Timeout: defaultFtpTimeout,
		Retry:   defaultFtpRetry,
	}
}

type Ftp struct {
	RW
	*FtpCfg
}

func NewFtp() RW {
	return &Ftp{
		RW: CreateBase(string(TypeFtp)),
	}
}

func (f *Ftp) Init() error {
	if f.IsStarter() {
		return errs.New("ftp rw must not be Starter")
	}

	if f.IsReader() {
		r, err := createFtpReader(f.FtpCfg)
		if err != nil {
			return errs.Wrap(err, "create ftp reader failed")
		}
		f.NestReader(r)
	} else {
		w, err := createFtpWriter(f.FtpCfg)
		if err != nil {
			return errs.Wrap(err, "create ftp writer failed")
		}
		f.NestWriter(w)
	}

	return f.RW.Init()
}

func (f *Ftp) Type() Type {
	return TypeFtp
}

func createFtpCfg(ftpCfg *FtpCfg) *ftp.Cfg {
	return &ftp.Cfg{
		Addr:    ftpCfg.Addr,
		User:    ftpCfg.User,
		Pwd:     ftpCfg.Pwd,
		Timeout: ftpCfg.Timeout,
		Retry:   ftpCfg.Retry,
	}
}

func createFtpReader(ftpCfg *FtpCfg) (*ftp.Reader, error) {
	r := ftp.NewReader()
	r.Cfg = createFtpCfg(ftpCfg)
	r.Path = ftpCfg.Path

	err := r.Init()
	if err != nil {
		return nil, errs.Wrap(err, "init ftp reader failed")
	}

	return r, nil
}

func createFtpWriter(ftpCfg *FtpCfg) (*ftp.Writer, error) {
	w := ftp.NewWriter()
	w.Cfg = createFtpCfg(ftpCfg)
	w.Path = ftpCfg.Path

	err := w.Init()
	if err != nil {
		return nil, errs.Wrap(err, "init ftp writer failed")
	}

	return w, nil
}

package pipeline

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/ftp"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(RWTypeFtp, func() any { return NewFtpRW() }, func() any { return NewFtpRWCfg() })
}

const (
	RWTypeFtp RWType = "ftp"

	defaultFtpTimeout = 600
	defaultFtpRetry   = 3
)

type FtpRWCfg struct {
	Addr    string `json:"addr"    validate:"required" yaml:"addr"`
	User    string `json:"user"    validate:"required" yaml:"user"`
	Pwd     string `json:"pwd"     validate:"required" yaml:"pwd"`
	Path    string `json:"path"    validate:"required" yaml:"path"`
	Timeout int    `json:"timeout" yaml:"timeout"`
	Retry   int    `json:"retry"   yaml:"retry"`
}

func NewFtpRWCfg() *FtpRWCfg {
	return &FtpRWCfg{
		Timeout: defaultFtpTimeout,
		Retry:   defaultFtpRetry,
	}
}

type FtpRW struct {
	RW
	*FtpRWCfg
}

func NewFtpRW() RW {
	return &FtpRW{
		RW: CreateBaseRW(string(RWTypeFtp)),
	}
}

func (f *FtpRW) Init() error {
	if f.IsStarter() {
		return errs.New("ftp rw must not be Starter")
	}

	if f.IsReader() {
		r, err := createFtpReader(f.FtpRWCfg)
		if err != nil {
			return errs.Wrap(err, "create ftp reader failed")
		}
		f.NestReader(r)
	} else {
		w, err := createFtpWriter(f.FtpRWCfg)
		if err != nil {
			return errs.Wrap(err, "create ftp writer failed")
		}
		f.NestWriter(w)
	}

	f.HookRead(f.hookLogRead)
	f.HookWrite(f.hookLogWrite)
	return f.RW.Init()
}

func (f *FtpRW) Type() any {
	return RWTypeFtp
}

func (f *FtpRW) GetCfg() any {
	return f.FtpRWCfg
}

func (f *FtpRW) hookLogWrite(n int, bs []byte, err error, cost int64, misc ...any) error {
	f.Info("write", "bs_len", len(bs), "bs_cap", cap(bs), "nw", n, "cost", cost,
		"async_chan_len", f.AsyncChanLen(), "async_chan_cap", f.AsyncChanCap(), "misc", misc, "err", err)
	return nil
}

func (f *FtpRW) hookLogRead(n int, bs []byte, err error, cost int64, misc ...any) error {
	f.Info("read", "bs_len", len(bs), "bs_cap", cap(bs), "nr", n, "cost", cost,
		"async_chan_len", f.AsyncChanLen(), "async_chan_cap", f.AsyncChanCap(), "misc", misc, "err", err)
	return nil
}

func createFtpCfg(ftpCfg *FtpRWCfg) *ftp.Cfg {
	return &ftp.Cfg{
		Addr:    ftpCfg.Addr,
		User:    ftpCfg.User,
		Pwd:     ftpCfg.Pwd,
		Timeout: ftpCfg.Timeout,
		Retry:   ftpCfg.Retry,
	}
}

func createFtpReader(ftpCfg *FtpRWCfg) (*ftp.Reader, error) {
	r := ftp.NewReader()
	r.Cfg = createFtpCfg(ftpCfg)
	r.Path = ftpCfg.Path

	err := r.Init()
	if err != nil {
		return nil, errs.Wrap(err, "init ftp reader failed")
	}

	return r, nil
}

func createFtpWriter(ftpCfg *FtpRWCfg) (*ftp.Writer, error) {
	w := ftp.NewWriter()
	w.Cfg = createFtpCfg(ftpCfg)
	w.Path = ftpCfg.Path

	err := w.Init()
	if err != nil {
		return nil, errs.Wrap(err, "init ftp writer failed")
	}

	return w, nil
}

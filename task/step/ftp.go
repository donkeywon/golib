package step

import (
	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/ftp"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/v"
)

func init() {
	plugin.RegWithCfg(TypeFtp, func() Step { return NewFtpStep() }, func() any { return NewFtpStepCfg })
}

const TypeFtp Type = "ftp"

type FtpStepCfg struct {
	Addr    string `json:"addr"    yaml:"addr" validate:"required"`
	User    string `json:"user"    yaml:"user" validate:"required"`
	Pwd     string `json:"pwd"     yaml:"pwd"`
	Timeout int    `json:"timeout" yaml:"timeout"`
	Retry   int    `json:"retry"   yaml:"retry"`

	Cmd  string   `json:"cmd"     yaml:"cmd"     validate:"required"`
	Args []string `json:"args"    yaml:"args"`
}

func NewFtpStepCfg() *FtpStepCfg {
	return &FtpStepCfg{}
}

type FtpStep struct {
	Step
	*FtpStepCfg

	cli *ftp.Client
}

func NewFtpStep() *FtpStep {
	return &FtpStep{
		Step: CreateBase(string(TypeFtp)),
	}
}

func (f *FtpStep) Init() error {
	err := v.Struct(f.FtpStepCfg)
	if err != nil {
		return err
	}

	f.WithLoggerFields("cmd", f.FtpStepCfg.Cmd, "addr", f.FtpStepCfg.Addr, "user", f.FtpStepCfg.User)
	return f.Step.Init()
}

func (f *FtpStep) Start() error {
	f.cli = ftp.NewClient()
	f.cli.Cfg = &ftp.Cfg{
		Addr:    f.FtpStepCfg.Addr,
		User:    f.FtpStepCfg.User,
		Pwd:     f.FtpStepCfg.Pwd,
		Timeout: f.FtpStepCfg.Timeout,
		Retry:   f.FtpStepCfg.Retry,
	}
	if f.cli.Cfg.Timeout <= 0 {
		f.cli.Cfg.Timeout = 30
	}
	if f.cli.Cfg.Retry <= 0 {
		f.cli.Cfg.Retry = 3
	}

	err := f.cli.Init()
	if err != nil {
		return errs.Wrap(err, "init ftp client failed")
	}

	defer func() {
		select {
		case <-f.Stopping():
			return
		default:
		}
		err = f.cli.Close()
		if err != nil {
			f.Error("close ftp client failed", err)
		}
	}()

	code, msg, err := f.cli.RawCmd(f.Cmd, f.Args)
	f.Store(consts.FieldFtpCode, code)
	f.Store(consts.FieldFtpMsg, msg)
	f.Info("ftp cmd done", "code", code, "msg", msg, "err", err)
	if err != nil {
		return errs.Wrap(err, "ftp cmd failed")
	}
	return nil
}

func (f *FtpStep) Stop() error {
	return f.cli.Close()
}

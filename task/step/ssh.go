package step

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/logs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/bufferpool"
	"github.com/donkeywon/golib/util/sshs"
	"github.com/donkeywon/golib/util/v"
	"golang.org/x/crypto/ssh"
)

func init() {
	plugin.Reg(TypeSSH, func() Step { return NewSSHStep() }, func() any { return NewSSHStepCfg() })
}

const TypeSSH Type = "ssh"

type SSHStepCfg struct {
	Addr       string        `json:"addr"       yaml:"addr" validate:"required"`
	User       string        `json:"user"       yaml:"user" validate:"required"`
	Pwd        string        `json:"pwd"        yaml:"pwd"  validate:"required"`
	PrivateKey string        `json:"privateKey" yaml:"privateKey"`
	Timeout    time.Duration `json:"timeout"    yaml:"timeout"`

	Cmd string `json:"cmd"  yaml:"cmd" validate:"required"`
}

func NewSSHStepCfg() SSHStepCfg {
	return SSHStepCfg{}
}

type SSHStep struct {
	Base

	cfg  SSHStepCfg
	cli  *ssh.Client
	sess *ssh.Session
}

func NewSSHStep() *SSHStep {
	return &SSHStep{}
}

func (s *SSHStep) SetCfg(cfg any) {
	s.cfg = cfg.(SSHStepCfg)
}

func (s *SSHStep) Init(ctx context.Context) error {
	err := v.Struct(s.cfg)
	if err != nil {
		return err
	}

	return nil
}

func (s *SSHStep) Start(ctx context.Context) error {
	l := logs.FromCtx(ctx).With("addr", s.cfg.Addr, "user", s.cfg.User, "cmd", s.cfg.Cmd)

	var err error
	s.cli, s.sess, err = sshs.NewClient(s.cfg.Addr, s.cfg.User, s.cfg.Pwd, []byte(s.cfg.PrivateKey), s.cfg.Timeout)
	if err != nil {
		return errs.Wrap(err, "create ssh client failed")
	}

	defer func() {
		err = sshs.Close(s.cli, s.sess)
		if err != nil {
			l.Error("close ssh client failed", "err", err)
		}
	}()

	stdoutBuf := bufferpool.Get()
	defer stdoutBuf.Free()
	stderrBuf := bufferpool.Get()
	defer stderrBuf.Free()

	startNano := time.Now().UnixNano()
	err = s.sess.Run(s.cfg.Cmd)
	stopNano := time.Now().UnixNano()
	s.Store(consts.FieldStartTimeNano, startNano)
	s.Store(consts.FieldStopTimeNano, stopNano)

	if err != nil {
		l.Error("ssh cmd failed", "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "cost_nano", stopNano-startNano, "err", err)
		return errs.Wrap(err, "ssh cmd failed")
	} else {
		l.Info("ssh cmd done", "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "cost_nano", stopNano-startNano)
	}
	return nil
}

func (s *SSHStep) Stop(ctx context.Context) error {
	if s.sess == nil {
		return nil
	}
	err := s.sess.Signal(ssh.SIGKILL)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

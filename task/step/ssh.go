package step

import (
	"errors"
	"io"
	"strings"
	"time"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/bufferpool"
	"github.com/donkeywon/golib/util/sshs"
	"github.com/donkeywon/golib/util/v"
	"golang.org/x/crypto/ssh"
)

func init() {
	plugin.RegWithCfg(TypeSSH, func() Step { return NewSSHStep() }, func() any { return NewSSHStepCfg() })
}

const TypeSSH Type = "ssh"

type SSHStepCfg struct {
	Addr       string `json:"addr"       yaml:"addr" validate:"required"`
	User       string `json:"user"       yaml:"user" validate:"required"`
	Pwd        string `json:"pwd"        yaml:"pwd"  validate:"required"`
	PrivateKey string `json:"privateKey" yaml:"privateKey"`
	Timeout    int    `json:"timeout"    yaml:"timeout"`

	Cmd []string `json:"cmd"  yaml:"cmd" validate:"required"`
}

func NewSSHStepCfg() *SSHStepCfg {
	return &SSHStepCfg{}
}

type SSHStep struct {
	Step
	*SSHStepCfg

	cli  *ssh.Client
	sess *ssh.Session
}

func NewSSHStep() *SSHStep {
	return &SSHStep{
		Step: CreateBase(string(TypeSSH)),
	}
}

func (s *SSHStep) Init() error {
	err := v.Struct(s.SSHStepCfg)
	if err != nil {
		return err
	}

	s.WithLoggerFields("addr", s.SSHStepCfg.Addr, "user", s.SSHStepCfg.User, "cmd", s.SSHStepCfg.User)
	return s.Step.Init()
}

func (s *SSHStep) Start() error {
	var err error
	s.cli, s.sess, err = sshs.NewClient(s.SSHStepCfg.Addr, s.SSHStepCfg.User, s.SSHStepCfg.Pwd, []byte(s.SSHStepCfg.PrivateKey), time.Second*time.Duration(s.SSHStepCfg.Timeout))
	if err != nil {
		return errs.Wrap(err, "create ssh client failed")
	}

	defer func() {
		err = sshs.Close(s.cli, s.sess)
		if err != nil {
			s.Error("close ssh client failed", err)
		}
	}()

	stdoutBuf := bufferpool.Get()
	defer stdoutBuf.Free()
	stderrBuf := bufferpool.Get()
	defer stderrBuf.Free()

	cmd := strings.Join(s.SSHStepCfg.Cmd, " ")

	startNano := time.Now().UnixNano()
	err = s.sess.Run(cmd)
	stopNano := time.Now().UnixNano()
	s.Store(consts.FieldStartTimeNano, startNano)
	s.Store(consts.FieldStopTimeNano, stopNano)
	s.Info("ssh cmd done", "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "cost_nano", stopNano-startNano, "err", err)
	if err != nil {
		return errs.Wrap(err, "ssh cmd failed")
	}
	return nil
}

func (s *SSHStep) Stop() error {
	err := s.sess.Signal(ssh.SIGKILL)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return nil
}

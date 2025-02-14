package rw

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
	plugin.RegWithCfg(TypeSSH, func() any { return NewSSH() }, func() any { return NewSSHCfg() })
}

const (
	TypeSSH Type = "ssh"

	defaultSSHTimeout = 30
)

type SSHCfg struct {
	Addr       string `json:"addr"       validate:"required"  yaml:"addr"`
	User       string `json:"user"       validate:"required"  yaml:"user"`
	Pwd        string `json:"pwd"        yaml:"pwd"`
	PrivateKey string `json:"privateKey" yaml:"privateKey"`
	Timeout    int    `json:"timeout"    yaml:"timeout"`
	Path       string `json:"path"       validate:"required"  yaml:"path"`
}

func NewSSHCfg() *SSHCfg {
	return &SSHCfg{
		Timeout: defaultSSHTimeout,
	}
}

type SSH struct {
	RW
	*SSHCfg

	sshCmd       string
	sshCli       *ssh.Client
	sshSess      *ssh.Session
	sshStderrBuf *bufferpool.Buffer
}

func NewSSH() RW {
	return &SSH{
		RW: CreateBase(string(TypeSSH)),
	}
}

func (s *SSH) Init() error {
	if !s.IsStarter() {
		return errs.New("ssh rw must be Starter")
	}

	var err error
	s.sshCli, s.sshSess, err = sshs.NewClient(s.SSHCfg.Addr, s.SSHCfg.User, s.SSHCfg.Pwd, []byte(s.SSHCfg.PrivateKey), s.SSHCfg.Timeout)
	if err != nil {
		return errs.Wrap(err, "create ssh cli and session failed")
	}
	if s.Reader() != nil {
		s.sshCmd = sshWriteCmd(s.SSHCfg.Path)
		s.sshSess.Stdin = s
	} else if s.Writer() != nil {
		s.sshCmd = sshReadCmd(s.SSHCfg.Path)
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

func (s *SSH) Start() error {
	err := s.sshSess.Run(s.sshCmd)
	exitError := &ssh.ExitError{}
	if errors.As(err, &exitError) {
		s.Info("exit signaled", "signal", exitError.Signal)
		err = nil
	}

	if err != nil {
		return errs.Wrapf(err, "ssh cmd failed: %s", s.sshCmd)
	}
	return nil
}

func (s *SSH) Close() error {
	return errors.Join(sshs.Close(s.sshCli, s.sshSess), s.RW.Close())
}

func (s *SSH) Stop() error {
	return s.sshSess.Signal(ssh.SIGKILL)
}

func (s *SSH) Type() any {
	return TypeSSH
}

func (s *SSH) GetCfg() any {
	return s.SSHCfg
}

func (s *SSH) hookLogWrite(n int, bs []byte, err error, cost int64, misc ...any) error {
	s.Info("write", "bs_len", len(bs), "bs_cap", cap(bs), "nw", n, "cost", cost,
		"async_chan_len", s.AsyncChanLen(), "async_chan_cap", s.AsyncChanCap(), "misc", misc, "err", err)
	return nil
}

func (s *SSH) hookLogRead(n int, bs []byte, err error, cost int64, misc ...any) error {
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

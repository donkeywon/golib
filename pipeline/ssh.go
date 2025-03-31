package pipeline

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/bufferpool"
	"github.com/donkeywon/golib/util/sshs"
	"golang.org/x/crypto/ssh"
)

func init() {
	plugin.RegWithCfg(WorkerSSH, func() Worker { return NewSSH() }, func() any { return NewSSHCfg() })
}

const WorkerSSH Type = "ssh"

type SSHCfg struct {
	Addr       string `json:"addr"       yaml:"addr" validate:"required"`
	User       string `json:"user"       yaml:"user" validate:"required"`
	Pwd        string `json:"pwd"        yaml:"pwd"`
	PrivateKey string `json:"privateKey" yaml:"privateKey"`
	Path       string `json:"path"       yaml:"path" validate:"required"`
	Timeout    int    `json:"timeout"    yaml:"timeout"`
}

func NewSSHCfg() *SSHCfg {
	return &SSHCfg{}
}

type SSH struct {
	Worker

	c            *SSHCfg
	sshCli       *ssh.Client
	sshSess      *ssh.Session
	sshStderrBuf *bufferpool.Buffer
	sshCmd       string
}

func NewSSH() *SSH {
	return &SSH{
		Worker: CreateWorker(string(WorkerSSH)),
		c:      NewSSHCfg(),
	}
}

func (s *SSH) Init() error {
	var err error
	s.sshCli, s.sshSess, err = sshs.NewClient(s.c.Addr, s.c.User, s.c.Pwd, []byte(s.c.PrivateKey), time.Second*time.Duration(s.c.Timeout))
	if err != nil {
		return errs.Wrap(err, "failed to create ssh client and session")
	}

	if s.Reader() != nil {
		s.sshCmd = sshWriteCmd(s.c.Path)
		s.sshSess.Stdin = s.Reader()
	} else if s.Writer() != nil {
		s.sshCmd = sshReadCmd(s.c.Path)
		s.sshSess.Stdout = s.Writer()
	} else {
		return errs.Errorf("ssh must has Reader or Writer")
	}

	return s.Worker.Init()
}

func (s *SSH) Start() error {
	s.sshStderrBuf = bufferpool.Get()
	defer s.sshStderrBuf.Free()
	s.sshSess.Stderr = s.sshStderrBuf

	err := s.sshSess.Run(s.sshCmd)
	exitError := &ssh.ExitError{}
	if errors.As(err, &exitError) {
		s.Info("ssh exit signaled", "signal", exitError.Signal)
		err = nil
	}

	if err != nil {
		return errs.Wrapf(err, "ssh cmd failed: %s", s.sshCmd)
	}
	return nil
}

func (s *SSH) Stop() error {
	err := s.sshSess.Signal(ssh.SIGKILL)
	if err == io.EOF {
		return nil
	}
	return errs.Wrap(err, "ssh signal kill failed")
}

func (s *SSH) Close() error {
	return errors.Join(sshs.Close(s.sshCli, s.sshSess), s.Worker.Close())
}

func (s *SSH) SetCfg(c any) {
	s.c = c.(*SSHCfg)
}

func sshReadCmd(path string) string {
	return "cat " + path
}

func sshWriteCmd(path string) string {
	dir := filepath.Dir(path)
	return fmt.Sprintf("mkdir -p %s; cat > %s", dir, path)
}

package sshs

import (
	"errors"
	"io"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/bufferpool"
	"golang.org/x/crypto/ssh"
)

const defaultTimeout = time.Minute

func NewClient(addr, user, pwd string, privateKey []byte, timeout time.Duration) (*ssh.Client, *ssh.Session, error) {
	cfg := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	cfg.Timeout = timeout
	if timeout == 0 {
		cfg.Timeout = defaultTimeout
	}

	if len(privateKey) > 0 {
		signer, err := ssh.ParsePrivateKey(privateKey)
		if err != nil {
			return nil, nil, errs.Wrap(err, "private key is invalid")
		}

		cfg.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		}
	} else {
		cfg.Auth = []ssh.AuthMethod{
			ssh.Password(pwd),
		}
	}

	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, nil, errs.Wrap(err, "ssh connect failed")
	}

	sess, err := client.NewSession()
	if err != nil {
		return nil, nil, errors.Join(client.Close(), errs.Wrap(err, "create ssh session failed"))
	}
	return client, sess, nil
}

func Close(cli *ssh.Client, sess *ssh.Session) error {
	// sess.Close may return io.EOF

	var err error
	sessErr := sess.Close()
	if sessErr != nil && !errors.Is(sessErr, io.EOF) {
		err = errors.Join(err, sessErr)
	}
	cliErr := cli.Close()
	if cliErr != nil && !errors.Is(cliErr, io.EOF) {
		err = errors.Join(err, cliErr)
	}

	if err != nil {
		return errs.Wrap(err, "close ssh client failed")
	}
	return nil
}

func Exec(addr, user, pwd string, privateKey []byte, timeout time.Duration, cmd string) (stdout string, stderr string, err error) {
	var (
		cli  *ssh.Client
		sess *ssh.Session
	)
	cli, sess, err = NewClient(addr, user, pwd, privateKey, timeout)
	if err != nil {
		return
	}
	defer cli.Close()
	defer sess.Close()

	stdoutBuf := bufferpool.Get()
	defer stdoutBuf.Free()
	stderrBuf := bufferpool.Get()
	defer stderrBuf.Free()
	sess.Stdout = stdoutBuf
	sess.Stderr = stderrBuf

	err = sess.Run(cmd)
	if err != nil {
		return
	}

	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()
	return
}

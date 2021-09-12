package sshs

import (
	"errors"
	"io"
	"time"

	"github.com/donkeywon/golib/errs"
	"golang.org/x/crypto/ssh"
)

const defaultTimeout = 30

func NewClient(addr, user, pwd string, privateKey []byte, timeout int) (*ssh.Client, *ssh.Session, error) {
	cfg := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	cfg.Timeout = time.Second * defaultTimeout
	if timeout > 0 {
		cfg.Timeout = time.Second * time.Duration(timeout)
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
		return nil, nil, errs.Wrap(err, "ssh connect fail")
	}

	sess, err := client.NewSession()
	if err != nil {
		return nil, nil, errors.Join(client.Close(), errs.Wrap(err, "ssh create session fail"))
	}
	return client, sess, nil
}

func Close(cli *ssh.Client, sess *ssh.Session) error {
	// sess.Signal and sess.Close may return io.EOF

	var err error
	sigErr := sess.Signal(ssh.SIGTERM)
	if sigErr != nil && !errors.Is(sigErr, io.EOF) {
		err = sigErr
	}
	sessErr := sess.Close()
	if sessErr != nil && !errors.Is(sessErr, io.EOF) {
		err = errors.Join(err, sessErr)
	}
	cliErr := cli.Close()
	if cliErr != nil && !errors.Is(cliErr, io.EOF) {
		err = errors.Join(err, cliErr)
	}

	if err != nil {
		return errs.Wrap(err, "close ssh client fail")
	}
	return nil
}

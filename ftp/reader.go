package ftp

import (
	"errors"
	"net"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/donkeywon/golib/errs"
)

type Reader struct {
	*Client
	Path string

	dataConn net.Conn
}

func NewReader() *Reader {
	return &Reader{
		Client: NewClient(),
	}
}

func (r *Reader) Init() error {
	err := r.Client.Init()
	if err != nil {
		return errs.Wrap(err, "init ftp client fail")
	}

	err = retry.Do(
		func() error {
			err = r.TransType("I")
			if err != nil {
				return errs.Wrap(err, "change transfer type fail")
			}

			r.dataConn, err = r.Client.cmdDataConn("RETR %s", r.Path)
			if err != nil {
				return errs.Wrap(err, "RETR fail")
			}
			return nil
		},
		retry.Attempts(uint(r.Retry)),
		retry.Delay(time.Second),
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reader) Read(b []byte) (int, error) {
	return r.dataConn.Read(b)
}

func (r *Reader) Close() error {
	return errors.Join(r.dataConn.Close(), r.Client.checkDataShut(), r.Client.Close())
}

package ftp

import (
	"errors"
	"net"
	"path/filepath"

	"github.com/donkeywon/golib/errs"
)

type Writer struct {
	*Client
	Path string

	dataConn net.Conn
}

func NewWriter() *Writer {
	return &Writer{
		Client: NewClient(),
	}
}

func (w *Writer) Init() error {
	err := w.Client.Init()
	if err != nil {
		return errs.Wrap(err, "init ftp client fail")
	}

	err = w.Client.TransType("I")
	if err != nil {
		return errs.Wrap(err, "change transfer type fail")
	}

	err = w.Client.MkdirRecur(filepath.Dir(w.Path))
	if err != nil {
		return errs.Wrap(err, "mkdir fail")
	}

	w.dataConn, err = w.Client.cmdDataConn("STOR %s", filepath.Base(w.Path))
	if err != nil {
		return errs.Wrap(err, "STOR fail")
	}

	return nil
}

func (w *Writer) Write(b []byte) (int, error) {
	return w.dataConn.Write(b)
}

func (w *Writer) Close() error {
	return errors.Join(w.dataConn.Close(), w.Client.Close())
}

package errs

import (
	"errors"
	"testing"

	"github.com/donkeywon/golib/util/bufferpool"
)

func errA() error {
	return errors.New("errA")
}

func errB() error {
	e := errA()
	_ = e
	return Wrap(e, "errB")
}

func errC() error {
	e := errB()
	_ = e
	return Wrap(e, "errC")
}

func errD() error {
	e := errC()
	_ = e
	return Wrap(e, "errD")
}

func errE() error {
	e1 := errD()
	e2 := errC()
	return Wrap(errors.Join(e1, e2), "errE")
}

func errF() error {
	e := errE()
	_ = e
	return Wrap(e, "errF")
}

func TestErr(t *testing.T) {
	err := errF()
	t.Logf("%+v", err)
}

func TestFormat(t *testing.T) {
	buf := bufferpool.GetBuffer()
	defer buf.Free()
	e := errC()
	ErrToStack(e, buf, 0)
	t.Log(buf.String())
}

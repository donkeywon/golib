package errs

import (
	"errors"
	"fmt"
	"testing"
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
	fmt.Printf("%+v", err)
}

func TestFormat(t *testing.T) {
	buf := getBuffer()
	defer buf.free()
	e := errC()
	ErrToStack(e, buf, 0)
	t.Log(buf.String())
}

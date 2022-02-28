package errs

import (
	"errors"
	"fmt"
	"testing"
)

func nestErr(i, max int) error {
	if i == max {
		return fmt.Errorf("err%d", i)
	}
	return Wrapf(nestErr(i+1, max), "err%d", i)
}

func errS() error {
	e1 := nestErr(0, 1)
	e2 := nestErr(0, 2)
	return Wrap(errors.Join(e1, e2), "errS")
}

func TestErr(t *testing.T) {
	err := errS()
	fmt.Printf("%+v", err)
}

func TestFormat(t *testing.T) {
	buf := getBuffer()
	defer buf.free()
	e := nestErr(0, 3)
	ErrToStack(e, buf, 0)
	t.Log(buf.String())
}

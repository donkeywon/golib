package log

import (
	"errors"
	"testing"

	"github.com/donkeywon/golib/errs"
	"github.com/stretchr/testify/require"
)

func errA() error {
	return errors.New("errA")
}

func errB() error {
	e := errA()
	_ = e
	return errs.Wrap(e, "errB")
}

func errC() error {
	e := errB()
	_ = e
	return errs.Wrap(e, "errC")
}

func errD() error {
	e := errC()
	_ = e
	return errs.Wrap(e, "errD")
}

func errE() error {
	e1 := errD()
	e2 := errC()
	return errs.Wrap(errors.Join(e1, e2), "errE")
}

func errF() error {
	e := errE()
	_ = e
	return errs.Wrap(e, "errF")
}

func TestLogErr(t *testing.T) {
	lc := NewCfg()
	lc.Filepath = "stdout"
	l, err := lc.Build()
	require.NoError(t, err)

	l.Error("error occurred", errF())
}

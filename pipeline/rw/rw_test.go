package rw

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/log"
	"github.com/donkeywon/golib/util/httpc"
	"io"
	"os"
	"testing"
	"time"

	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

func setup() {
}

func teardown() {
}

func TestMain(m *testing.M) {
	setup()
	exit := m.Run()
	teardown()
	os.Exit(exit)
}

type mockReader struct {
	limit int
}

func (m *mockReader) Read(b []byte) (int, error) {
	var nr int
	for i := 0; i < len(b); i++ {
		if m.limit == 0 {
			return nr, io.EOF
		}

		b[i] = '0'
		m.limit--
		nr++
	}
	return nr, nil
}

func (m *mockReader) Close() error {
	return nil
}

func TestSimpleRead(t *testing.T) {
	rw := NewNop()
	rw.NestReader(&mockReader{limit: 15})

	rw.AsReader()
	rw.EnableReadBuf(100, false, 10)

	err := runner.Init(rw)
	require.NoError(t, err)

	bs := make([]byte, 10)
	nr, err := rw.Read(bs)
	require.Equal(t, 10, nr)
	require.NoError(t, err)

	bs = make([]byte, 10)
	nr, err = rw.Read(bs)
	require.Equal(t, 5, nr)
	require.ErrorIs(t, err, io.EOF)
}

func TestAsyncRead(t *testing.T) {
	rw := NewNop()
	rw.NestReader(&mockReader{limit: 15})

	rw.AsReader()
	rw.EnableReadBuf(6, true, 5)
	tests.DebugInit(rw)

	err := runner.Init(rw)
	require.NoError(t, err)

	time.Sleep(time.Second)
	require.Equal(t, 3, rw.AsyncChanLen())
}

func do() error {
	_, err := httpc.Get(nil, time.Second, "http://127.0.0.1:5678")
	if err != nil {
		return errs.Wrap(err, "fetch meta fail")
	}
	return nil
}

func TestMeta(t *testing.T) {
	err := do()
	if err != nil {
		log.Default().Error("fail", err)
	}
}

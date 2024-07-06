package pipeline

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/test"
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
	rw := NewNopRW()
	err := rw.NestReader(&mockReader{limit: 15})
	require.NoError(t, err)

	rw.AsReader()
	rw.EnableReadBuf(100, false, 10)

	err = runner.Init(rw)
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
	rw := NewNopRW()
	err := rw.NestReader(&mockReader{limit: 15})
	require.NoError(t, err)

	rw.AsReader()
	rw.EnableReadBuf(6, true, 5)
	test.DebugInherit(rw)

	err = runner.Init(rw)
	require.NoError(t, err)

	time.Sleep(time.Second)
	require.Equal(t, 3, rw.AsyncChanLen())
}

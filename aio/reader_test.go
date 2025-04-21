package aio

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var errTest = errors.New("test error")

type testReader struct {
	err        error
	errOnCount int
	errOnBytes int
	nrPerRead  int
	count      int
	nr         int
}

func (r *testReader) Read(p []byte) (n int, err error) {
	if r.count == r.errOnCount {
		return 0, r.err
	}
	if r.nr >= r.errOnBytes {
		return 0, r.err
	}
	r.count++
	nrPerRead := min(r.nrPerRead, r.errOnBytes-r.nr, len(p))
	for i := range nrPerRead {
		p[i] = '1'
	}
	r.nr += nrPerRead
	if r.nr >= r.errOnBytes {
		return nrPerRead, r.err
	}

	return nrPerRead, nil
}

func TestReader(t *testing.T) {
	var (
		tr  *testReader
		ar  *AsyncReader
		p   []byte
		nr  int
		err error
	)

	tr = &testReader{
		errOnCount: 1,
		errOnBytes: 100,
		nrPerRead:  10,
		err:        io.EOF,
	}

	ar = NewAsyncReader(tr, BufSize(2), QueueSize(0))
	p = make([]byte, 3)
	nr, err = ar.Read(p)
	require.Equal(t, 2, nr)
	require.Equal(t, io.EOF, err)
	require.NoError(t, ar.Close())

	tr = &testReader{
		errOnCount: 1,
		errOnBytes: 3,
		nrPerRead:  10,
		err:        io.EOF,
	}

	ar = NewAsyncReader(tr, BufSize(4), QueueSize(0))
	p = make([]byte, 3)
	nr, err = ar.Read(p)
	require.Equal(t, 3, nr)
	require.Equal(t, nil, err)
	nr, err = ar.Read(p)
	require.Equal(t, 0, nr)
	require.Equal(t, io.EOF, err)
	require.NoError(t, ar.Close())

	tr = &testReader{
		errOnCount: 100,
		errOnBytes: 5,
		nrPerRead:  3,
		err:        io.EOF,
	}

	ar = NewAsyncReader(tr, BufSize(4), QueueSize(0))
	p = make([]byte, 3)
	nr, err = ar.Read(p)
	require.Equal(t, 3, nr)
	require.Equal(t, nil, err)
	nr, err = ar.Read(p)
	require.Equal(t, 2, nr)
	require.Equal(t, io.EOF, err)
	require.NoError(t, ar.Close())

	tr = &testReader{
		errOnCount: 100,
		errOnBytes: 80,
		nrPerRead:  30,
		err:        errTest,
	}

	ar = NewAsyncReader(tr, BufSize(4), QueueSize(2))
	p = make([]byte, 4)
	var nwi int
	nwi, err = ar.Read(p)
	time.Sleep(time.Millisecond * 10)
	require.Equal(t, 4, nwi)
	require.Equal(t, 2, len(ar.queue))
	require.Equal(t, 0, len(ar.bufChan))
	require.NoError(t, err)
	nwi, err = ar.Read(p)
	require.Equal(t, 4, nwi)
	require.Equal(t, 2, len(ar.queue))
	require.Equal(t, 1, len(ar.bufChan))
	require.NoError(t, err)
	require.NoError(t, ar.Close())

	tr = &testReader{
		errOnCount: 100,
		errOnBytes: 8,
		nrPerRead:  3,
		err:        errTest,
	}

	ar = NewAsyncReader(tr, BufSize(4), QueueSize(2))
	b := bytes.NewBuffer(nil)
	var nw int64
	nw, err = ar.WriteTo(b)
	require.Equal(t, int64(8), nw)
	require.Equal(t, errTest, err)
	require.NoError(t, ar.Close())
}

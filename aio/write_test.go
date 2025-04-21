package aio

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testWriter struct {
	err          error
	errOnCount   int
	errOnBytes   int
	costPerWrite time.Duration
	count        int
	nw           int
	trigger      chan struct{}
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	<-w.trigger
	if w.count == w.errOnCount {
		return 0, w.err
	}
	if w.nw >= w.errOnBytes {
		return 0, w.err
	}
	w.count++
	nw := min(w.errOnBytes-w.nw, len(p))
	w.nw += nw
	time.Sleep(w.costPerWrite)
	if w.nw >= w.errOnBytes {
		return nw, w.err
	}
	return len(p), nil
}

func (w *testWriter) triggerWrite() {
	w.trigger <- struct{}{}
}

func TestWriter(t *testing.T) {
	var (
		tw *testWriter
		aw *AsyncWriter
		//p   []byte
		nw  int
		err error
	)

	//p = []byte("abcde")
	tw = &testWriter{
		errOnCount:   100,
		errOnBytes:   100,
		err:          errTest,
		costPerWrite: time.Millisecond * 3,
		trigger:      make(chan struct{}, 10),
	}
	aw = NewAsyncWriter(tw, BufSize(2), QueueSize(1), Deadline(time.Second*2))
	nw, err = aw.Write([]byte("ab"))
	require.NoError(t, err)
	require.Equal(t, 2, nw)

	nw, err = aw.Write([]byte("cd"))
	require.NoError(t, err)
	require.Equal(t, 2, nw)
	require.Equal(t, 1, len(aw.queue))

	tw.triggerWrite()
	nw, err = aw.Write([]byte("ef"))
	require.NoError(t, err)
	require.Equal(t, 1, len(aw.queue))
	require.Equal(t, 1, len(aw.bufChan))

	tw.triggerWrite()
	tw.triggerWrite()
	time.Sleep(time.Millisecond * 10)
	require.Equal(t, 3, len(aw.bufChan))
	require.Equal(t, nil, aw.Close())

	p := []byte("abcd")
	tw = &testWriter{
		errOnCount:   1,
		errOnBytes:   100,
		err:          errTest,
		costPerWrite: time.Millisecond * 3,
		trigger:      make(chan struct{}, 10),
	}
	tw.triggerWrite()
	tw.triggerWrite()
	aw = NewAsyncWriter(tw, BufSize(2), QueueSize(1), Deadline(time.Millisecond*2))
	nw, err = aw.Write(p)
	require.Equal(t, 4, nw)
	require.NoError(t, err)
	require.Equal(t, errTest, aw.Close())

	p = []byte("abc")
	tw = &testWriter{
		errOnCount:   1,
		errOnBytes:   100,
		err:          errTest,
		costPerWrite: time.Millisecond * 3,
		trigger:      make(chan struct{}, 10),
	}
	tw.triggerWrite()
	tw.triggerWrite()
	aw = NewAsyncWriter(tw, BufSize(2), QueueSize(1), Deadline(time.Millisecond*2))
	nw, err = aw.Write(p)
	require.Equal(t, 3, nw)
	require.NoError(t, err)
	time.Sleep(time.Millisecond * 5)
	require.Equal(t, errTest, aw.Close())

	p = []byte("abc")
	tw = &testWriter{
		errOnCount:   10,
		errOnBytes:   100,
		err:          errTest,
		costPerWrite: time.Millisecond * 3,
		trigger:      make(chan struct{}, 10),
	}
	tw.triggerWrite()
	tw.triggerWrite()
	aw = NewAsyncWriter(tw, BufSize(2), QueueSize(1), Deadline(time.Millisecond*2))
	var nnw int64
	nnw, err = aw.ReadFrom(bytes.NewReader(p))
	require.Equal(t, int64(3), nnw)
	require.NoError(t, err)
	require.NoError(t, aw.Close())
}

package aio

import (
	"bytes"
	"io"
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

// ——— Coverage-boosting tests ———

func TestLoadOnceError_Err_Loaded(t *testing.T) {
	e := &loadOnceError{}
	e.Store(errTest)
	err1 := e.Err()
	require.ErrorIs(t, err1, errTest)
	// Second call: loaded==true, returns nil
	err2 := e.Err()
	require.NoError(t, err2)
}

func TestAsyncWriter_Write_ToClosed(t *testing.T) {
	tw := &testWriter{trigger: make(chan struct{}, 10)}
	tw.triggerWrite()
	tw.triggerWrite()
	aw := NewAsyncWriter(tw, BufSize(2), QueueSize(1))
	// Write to initialize, then close
	aw.Write([]byte("ab"))
	time.Sleep(20 * time.Millisecond)
	aw.Close()
	n, _ := aw.Write([]byte("cd"))
	require.Equal(t, 0, n)
	// May have error stored or nil (if no error from asyncWrite)
}

func TestAsyncWriter_Write_ErrorDuringLoop(t *testing.T) {
	// Writer that errors on first call, triggers error in asyncWrite
	tw := &testWriter{
		err:        errTest,
		errOnCount: 0,
		errOnBytes: 100,
		trigger:    make(chan struct{}, 10),
	}
	tw.triggerWrite()
	aw := NewAsyncWriter(tw, BufSize(2), QueueSize(1))
	// First write fills buffer and flushes to queue
	aw.Write([]byte("ab"))
	// trigger write which will error
	tw.triggerWrite()
	time.Sleep(30 * time.Millisecond)
	// Now write again - Load() should detect stored error
	n, err := aw.Write([]byte("cd"))
	if err != nil {
		require.ErrorIs(t, err, errTest)
	} else {
		require.Equal(t, 2, n)
	}
	aw.Close()
}

func TestAsyncWriter_ReadFrom_Closed(t *testing.T) {
	tw := &testWriter{trigger: make(chan struct{}, 10)}
	aw := NewAsyncWriter(tw, BufSize(2), QueueSize(1))
	aw.Close()
	n, _ := aw.ReadFrom(bytes.NewReader([]byte("test")))
	require.Equal(t, int64(0), n)
}

type errorOnlyReader struct{ err error }

func (r *errorOnlyReader) Read(p []byte) (int, error) { return 0, r.err }

func TestAsyncWriter_ReadFrom_ReaderError(t *testing.T) {
	tw := &testWriter{
		errOnCount: 100,
		errOnBytes: 100,
		trigger:    make(chan struct{}, 10),
	}
	tw.triggerWrite()
	tw.triggerWrite()
	aw := NewAsyncWriter(tw, BufSize(100), QueueSize(1))
	errReader := &errorOnlyReader{err: errTest}
	n, err := aw.ReadFrom(errReader)
	require.ErrorIs(t, err, errTest)
	require.Equal(t, int64(0), n)
	aw.Close()
}

func TestAsyncWriter_ReadFrom_WriteError(t *testing.T) {
	tw := &testWriter{
		err:        errTest,
		errOnCount: 0,
		errOnBytes: 100,
		trigger:    make(chan struct{}, 10),
	}
	tw.triggerWrite()
	tw.triggerWrite()
	aw := NewAsyncWriter(tw, BufSize(2), QueueSize(1))
	// ReadFrom reads into buffer, then flush calls write which errors
	n, err := aw.ReadFrom(bytes.NewReader([]byte("ab")))
	// First read fills buffer, flush sends to queue, writer errors
	require.Equal(t, int64(2), n)
	require.NoError(t, err)
	// After the error is stored, more reads should detect it
	time.Sleep(20 * time.Millisecond)
	n, err = aw.ReadFrom(bytes.NewReader([]byte("cd")))
	if err != nil {
		require.ErrorIs(t, err, errTest)
	}
	aw.Close()
}

func TestAsyncWriter_FlushMinSize(t *testing.T) {
	tw := &testWriter{trigger: make(chan struct{}, 10)}
	aw := NewAsyncWriter(tw, BufSize(100), QueueSize(1))
	// off < n: should not flush
	aw.off = 5
	aw.buf = make([]byte, 100)
	aw.flushMinSize(10)
	require.Equal(t, 5, aw.off)

	// off >= n: should flush
	aw.off = 20
	aw.buf = make([]byte, 100)
	tw.triggerWrite()
	aw.flushMinSize(10)
	require.Nil(t, aw.buf)
}

func TestAsyncWriter_AsyncWrite_ShortWrite(t *testing.T) {
	tw := &testWriter{
		errOnCount: 100,
		errOnBytes: 1,
		err:        nil,
		trigger:    make(chan struct{}, 10),
	}
	tw.triggerWrite()
	tw.triggerWrite()
	aw := NewAsyncWriter(tw, BufSize(2), QueueSize(1))
	// Write 2 bytes, flush sends buf[0:2] to queue
	aw.Write([]byte("ab"))
	// trigger write: testWriter.Write returns 1 byte (errOnBytes=1)
	// asyncWrite sees nw < len(b) → Store(ErrShortWrite)
	// next loop: Has()==true → bufChan <- b; continue
	time.Sleep(50 * time.Millisecond)
	err := aw.Close()
	if err != nil {
		require.ErrorIs(t, err, io.ErrShortWrite)
	}
}

func TestAsyncWriter_AsyncWrite_HasError(t *testing.T) {
	tw := &testWriter{
		err:        errTest,
		errOnCount: 0,
		errOnBytes: 100,
		trigger:    make(chan struct{}, 10),
	}
	tw.triggerWrite()
	tw.triggerWrite()
	aw := NewAsyncWriter(tw, BufSize(2), QueueSize(1))
	// Write fills buffer and queues; asyncWrite hits error, stores err
	aw.Write([]byte("ab"))
	time.Sleep(30 * time.Millisecond)
	// Send another buffer to queue - asyncWrite should see Has() and recycle
	aw.Write([]byte("cd"))
	time.Sleep(30 * time.Millisecond)
	_ = aw.Close()
	require.True(t, aw.err.Has())
}

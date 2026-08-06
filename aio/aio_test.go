package aio

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTestFail = errors.New("test fail")

type failWriter struct{}

func (f *failWriter) Write(p []byte) (int, error) {
	return 0, errTestFail
}

type shortWriter struct{}

func (s *shortWriter) Write(p []byte) (int, error) {
	return len(p) - 1, nil // always writes short, no error
}

type blockingWriter struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingWriter) Write(p []byte) (int, error) {
	b.once.Do(func() { close(b.started) })
	<-b.release
	return len(p), nil
}

func TestAsyncWriter_CloseFlushAll(t *testing.T) {
	var buf bytes.Buffer
	w := NewAsyncWriter(&buf, BufSize(128), QueueSize(4))
	data := strings.Repeat("flush me\n", 1000)

	n, err := w.Write([]byte(data))
	require.NoError(t, err)
	assert.Equal(t, len(data), n)

	require.NoError(t, w.Close())
	assert.Equal(t, data, buf.String())
}

func TestAsyncWriter_NoDeadline(t *testing.T) {
	// Default config: no deadline timer, flush on buffer full must not panic.
	var buf bytes.Buffer
	w := NewAsyncWriter(&buf, BufSize(8), QueueSize(2))
	data := strings.Repeat("x", 1024)

	n, err := w.Write([]byte(data))
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	require.NoError(t, w.Close())
	require.Equal(t, data, buf.String())
}

func TestAsyncWriter_CloseBeforeWrite(t *testing.T) {
	var buf bytes.Buffer
	w := NewAsyncWriter(&buf)
	require.NoError(t, w.Close()) // must not deadlock or panic
}

func TestAsyncWriter_WriteAfterClose(t *testing.T) {
	var buf bytes.Buffer
	w := NewAsyncWriter(&buf)
	require.NoError(t, w.Close())

	_, err := w.Write([]byte("x"))
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestAsyncWriter_CloseTwice(t *testing.T) {
	var buf bytes.Buffer
	w := NewAsyncWriter(&buf)
	require.NoError(t, w.Close())
	require.NoError(t, w.Close())
}

func TestAsyncWriter_ManualFlush(t *testing.T) {
	var buf bytes.Buffer
	w := NewAsyncWriter(&buf, BufSize(1<<20), QueueSize(2))

	_, err := w.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Empty(t, buf.String()) // buffered, not flushed yet

	require.NoError(t, w.Flush())
	require.Eventually(t, func() bool { return buf.String() == "hello" }, time.Second, time.Millisecond)
	require.NoError(t, w.Close())
}

func TestAsyncWriter_DeadlineFlush(t *testing.T) {
	var buf bytes.Buffer
	w := NewAsyncWriter(&buf, BufSize(1<<20), QueueSize(2), Deadline(10*time.Millisecond))

	_, err := w.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Empty(t, buf.String())

	require.Eventually(t, func() bool { return buf.String() == "hello" }, time.Second, 5*time.Millisecond)
	require.NoError(t, w.Close())
}

func TestAsyncWriter_DeadlineMinSizeSkips(t *testing.T) {
	var buf bytes.Buffer
	w := NewAsyncWriter(&buf, BufSize(1<<20), QueueSize(2), Deadline(10*time.Millisecond), DeadlineFlushMinSize(100))

	_, err := w.Write([]byte("short"))
	require.NoError(t, err)

	// Several deadline cycles pass, but the buffered data stays under min size.
	time.Sleep(60 * time.Millisecond)
	assert.Empty(t, buf.String())

	// Close flushes regardless of min size.
	require.NoError(t, w.Close())
	assert.Equal(t, "short", buf.String())
}

func TestAsyncWriter_ErrReportedByClose(t *testing.T) {
	// asyncWrite fails after Close's Flush: Close must still report the error
	// (it consumes the error only after waiting for asyncWriteDone).
	w := NewAsyncWriter(&failWriter{}, BufSize(8), QueueSize(2))

	_, err := w.Write([]byte("data"))
	require.NoError(t, err)

	err = w.Close()
	require.ErrorIs(t, err, errTestFail)
}

func TestAsyncWriter_ErrVisibleInWrite(t *testing.T) {
	w := NewAsyncWriter(&failWriter{}, BufSize(8), QueueSize(2))

	_, err := w.Write([]byte("0123456789ABCDEF")) // fills buffer, triggers async flush
	require.NoError(t, err)                       // async error not visible yet

	require.Eventually(t, func() bool {
		_, err := w.Write([]byte("x"))
		return err != nil
	}, time.Second, time.Millisecond)

	err = w.Close()
	require.ErrorIs(t, err, errTestFail)
}

func TestAsyncWriter_ConcurrentWrite(t *testing.T) {
	var buf bytes.Buffer
	w := NewAsyncWriter(&buf, BufSize(64), QueueSize(4))

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := strings.Repeat("abcd", 100)
			n, err := w.Write([]byte(data))
			assert.NoError(t, err)
			assert.Equal(t, len(data), n)
		}()
	}
	wg.Wait()
	require.NoError(t, w.Close())

	// Write order across goroutines is not guaranteed, but content must be intact.
	b := buf.Bytes()
	require.Equal(t, 4*400, len(b))
	for i := 0; i < len(b); i += 4 {
		assert.Equal(t, "abcd", string(b[i:i+4]), "content corrupted at offset %d", i)
	}
}

func TestAsyncWriter_ConcurrentCloseWrite(t *testing.T) {
	var buf bytes.Buffer
	w := NewAsyncWriter(&buf, BufSize(64), QueueSize(2))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_, _ = w.Write([]byte("concurrent"))
		}
	}()
	go func() {
		defer wg.Done()
		_ = w.Close()
	}()
	wg.Wait() // must not panic (send on closed queue)
}

func TestLoadOnceError_LoadedBranch(t *testing.T) {
	e := &loadOnceError{}
	e.Add(errTestFail)
	require.ErrorIs(t, e.LoadOnce(), errTestFail)
	require.NoError(t, e.LoadOnce()) // already loaded
}

func TestAsyncWriter_ShortWrite(t *testing.T) {
	w := NewAsyncWriter(&shortWriter{}, BufSize(8), QueueSize(2))

	_, err := w.Write([]byte("0123456789ABCDEF"))
	require.NoError(t, err)

	require.Eventually(t, func() bool { return w.asyncWriteErr.Has() }, time.Second, time.Millisecond)
	err = w.Close()
	require.ErrorIs(t, err, io.ErrShortWrite)
}

func TestAsyncWriter_AsyncErrSkipsRemaining(t *testing.T) {
	// After the first block fails, remaining queued blocks are skipped via
	// the asyncWriteErr.Has() path and returned to the pool.
	w := NewAsyncWriter(&failWriter{}, BufSize(8), QueueSize(4))

	_, err := w.Write([]byte("0123456789ABCDEF")) // two blocks enter the queue
	require.NoError(t, err)

	require.Eventually(t, func() bool { return w.asyncWriteErr.Has() }, time.Second, time.Millisecond)
	require.Eventually(t, func() bool { return len(w.queue) == 0 }, time.Second, time.Millisecond)

	err = w.Close()
	require.ErrorIs(t, err, errTestFail)
}

func TestAsyncWriter_FlushClosedDuringWrite(t *testing.T) {
	// Deterministically block a Write on a full queue: the asyncWrite
	// goroutine is stuck on a slow underlying writer, the queue is full, and
	// the in-flight flush must surface io.ErrClosedPipe instead of silently
	// dropping the buffered block.
	bw := &blockingWriter{started: make(chan struct{}), release: make(chan struct{})}
	w := NewAsyncWriter(bw, BufSize(8), QueueSize(1))

	done := make(chan struct{})
	var n int
	var werr error
	go func() {
		n, werr = w.Write([]byte(strings.Repeat("x", 1024)))
		close(done)
	}()

	<-bw.started // asyncWrite is stuck writing the first block
	// queue is full again (block 2), Write is blocked on block 3's flush
	require.Eventually(t, func() bool { return len(w.queue) == 1 }, time.Second, time.Millisecond)

	close(w.closed)
	w.mu.Lock()
	close(w.queue)
	w.mu.Unlock()

	<-done
	assert.ErrorIs(t, werr, io.ErrClosedPipe)
	assert.Greater(t, n, 0) // blocks that reached the queue are counted; the rest are reported as closed
	close(bw.release)
}

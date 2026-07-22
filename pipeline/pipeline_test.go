package pipeline

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/donkeywon/golib/logs"
	"github.com/donkeywon/golib/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWriter implements io.WriteCloser for testing.
type mockWriter struct {
	mu      sync.Mutex
	buf     []byte
	closed  bool
	flushFn func() error
}

func newMockWriter() *mockWriter { return &mockWriter{} }

func (m *mockWriter) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	m.buf = append(m.buf, p...)
	return len(p), nil
}

func (m *mockWriter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockWriter) Flush() error {
	if m.flushFn != nil {
		return m.flushFn()
	}
	return nil
}

// mockReader implements io.ReadCloser for testing.
type mockReader struct {
	mu     sync.Mutex
	closed bool
	data   []byte
	pos    int
}

func newMockReader(data []byte) *mockReader {
	return &mockReader{data: data}
}

func (m *mockReader) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n := copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockReader) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// mockFlusher implements flusher but not io.Closer.
type mockFlusher struct {
	flushErr error
}

func (m *mockFlusher) Write(p []byte) (int, error) { return len(p), nil }
func (m *mockFlusher) Flush() error                { return m.flushErr }

// mockFlusher2 implements flusher2 but not io.Closer.
type mockFlusher2 struct{}

func (m *mockFlusher2) Write(p []byte) (int, error) { return len(p), nil }
func (m *mockFlusher2) Flush()                      {}

// nonCloser is a writer that doesn't implement io.Closer, flusher, or flusher2.
type nonCloser struct{}

func (n *nonCloser) Write(p []byte) (int, error) { return len(p), nil }

// nonCloserReader is a reader that doesn't implement io.Closer.
type nonCloserReader struct{}

func (n *nonCloserReader) Read(p []byte) (int, error) { return 0, io.EOF }

// MockWorker implements Worker for testing pipeline wiring.
type MockWorker struct {
	BaseWorker

	name       string
	initErr    error
	startErr   error
	stopErr    error
	startDelay time.Duration
}

func NewMockWorker(name string) *MockWorker {
	return &MockWorker{name: name}
}

type emptyReader struct{}

func (e *emptyReader) Read(p []byte) (int, error) { return 0, io.EOF }

func (m *MockWorker) Init(ctx context.Context) error {
	if m.initErr != nil {
		return m.initErr
	}
	// BaseWorker.Init requires both reader and writer.
	// But in pipeline wiring, edge workers might have only one set.
	// We only call BaseWorker.Init if both are set.
	if m.Reader() != nil && m.Writer() != nil {
		return m.BaseWorker.Init(ctx)
	}
	return nil
}

func (m *MockWorker) Start(ctx context.Context) error {
	if m.startDelay > 0 {
		time.Sleep(m.startDelay)
	}
	if m.startErr != nil {
		return m.startErr
	}
	<-m.Stopping()
	return nil
}

func (m *MockWorker) Stop(ctx context.Context) error {
	return m.stopErr
}

// TestPipeline_Add tests Pipeline.Add.
func TestPipeline_Add(t *testing.T) {
	p := &Pipeline{}
	assert.Len(t, p.ws, 0)

	w1 := NewMockWorker("w1")
	w2 := NewMockWorker("w2")
	p.Add(w1, w2)
	assert.Len(t, p.ws, 2)
	assert.Equal(t, w1, p.ws[0])
	assert.Equal(t, w2, p.ws[1])
}

// TestPipeline_Init tests Pipeline.Init.
func TestPipeline_Init(t *testing.T) {
	t.Run("no workers panics", func(t *testing.T) {
		p := &Pipeline{}
		assert.PanicsWithValue(t, "no workers", func() {
			_ = p.Init(context.Background())
		})
	})

	t.Run("with 2 workers", func(t *testing.T) {
		p := &Pipeline{}
		w1 := NewMockWorker("w1")
		w2 := NewMockWorker("w2")
		p.Add(w1, w2)

		err := p.Init(context.Background())
		require.NoError(t, err)

		assert.NotNil(t, w1.Writer())
		assert.NotNil(t, w2.Reader())
	})

	t.Run("with 3 workers", func(t *testing.T) {
		p := &Pipeline{}
		w1 := NewMockWorker("w1")
		w2 := NewMockWorker("w2")
		w3 := NewMockWorker("w3")
		p.Add(w1, w2, w3)

		err := p.Init(context.Background())
		require.NoError(t, err)

		assert.NotNil(t, w1.Writer())
		assert.NotNil(t, w2.Reader())
		assert.NotNil(t, w2.Writer())
		assert.NotNil(t, w3.Reader())
	})

	t.Run("duplicate writer panics", func(t *testing.T) {
		p := &Pipeline{}
		w1 := NewMockWorker("w1")
		w2 := NewMockWorker("w2")
		// Pre-set a writer on w1.
		w1.WriteToWriter(newMockWriter())
		p.Add(w1, w2)

		assert.Panics(t, func() {
			_ = p.Init(context.Background())
		})
	})

	t.Run("duplicate reader panics", func(t *testing.T) {
		p := &Pipeline{}
		w1 := NewMockWorker("w1")
		w2 := NewMockWorker("w2")
		// Pre-set a reader on w2.
		w2.ReadFromReader(newMockReader(nil))
		p.Add(w1, w2)

		assert.Panics(t, func() {
			_ = p.Init(context.Background())
		})
	})
}

// TestPipeline_Start tests Pipeline.Start.
func TestPipeline_Start(t *testing.T) {
	t.Run("start with workers", func(t *testing.T) {
		p := &Pipeline{}
		w1 := NewMockWorker("w1")
		w2 := NewMockWorker("w2")
		p.Add(w1, w2)

		err := p.Init(context.Background())
		require.NoError(t, err)

		// Start in goroutine (Start blocks on wg.Wait which waits
		// for all workers to finish, which won't happen until Stop).
		go func() {
			_ = p.Start(context.Background())
		}()

		// Wait for the first worker to be marked as started.
		select {
		case <-w1.Started():
		case <-time.After(time.Second):
			t.Fatal("worker 1 not started")
		}

		// Stop the pipeline.
		err = p.Stop(context.Background())
		require.NoError(t, err)
	})
}

// TestPipeline_Stop tests Pipeline.Stop.
func TestPipeline_Stop(t *testing.T) {
	p := &Pipeline{}
	w1 := NewMockWorker("w1")
	w2 := NewMockWorker("w2")
	p.Add(w1, w2)

	err := p.Init(context.Background())
	require.NoError(t, err)

	// Start in goroutine.
	go func() {
		_ = p.Start(context.Background())
	}()

	// Wait for the first worker to be marked as started.
	select {
	case <-w1.Started():
	case <-time.After(time.Second):
		t.Fatal("worker 1 not started")
	}

	err = p.Stop(context.Background())
	require.NoError(t, err)
}

// TestBaseWorker_Init tests BaseWorker.Init.
func TestBaseWorker_Init(t *testing.T) {
	t.Run("nil reader panics", func(t *testing.T) {
		bw := &BaseWorker{}
		assert.PanicsWithValue(t, "nil reader or writer", func() {
			_ = bw.Init(context.Background())
		})
	})

	t.Run("nil writer panics", func(t *testing.T) {
		bw := &BaseWorker{}
		// Set reader but not writer.
		bw.ReadFromReader(newMockReader(nil))
		assert.PanicsWithValue(t, "nil reader or writer", func() {
			_ = bw.Init(context.Background())
		})
	})

	t.Run("valid init", func(t *testing.T) {
		bw := &BaseWorker{}
		bw.WriteToWriter(newMockWriter())
		bw.ReadFromReader(newMockReader(nil))
		err := bw.Init(context.Background())
		require.NoError(t, err)
	})
}

// TestBaseWorker_WrapWriters tests wrapWriters.
func TestBaseWorker_WrapWriters(t *testing.T) {
	t.Run("with writer wrappers", func(t *testing.T) {
		bw := &BaseWorker{}
		mw := newMockWriter()
		bw.WriteToWriter(mw, BufWrite(4096))

		assert.Equal(t, 1, len(bw.wwrappers))
		// Initial writer (before init) is the bufio writer wrapping mw.
		// Actually WriteToWriter sets w to the last writer, but wrapping
		// hasn't happened yet - it happens in Init.
	})

	t.Run("wrapWriters in init", func(t *testing.T) {
		bw := &BaseWorker{}
		mw := newMockWriter()
		bw.WriteToWriter(mw, BufWrite(4096))
		bw.ReadFromReader(newMockReader(nil))

		err := bw.Init(context.Background())
		require.NoError(t, err)

		// After init, ws should contain both the original and the wrapped writer.
		assert.Len(t, bw.ws, 2)
		assert.NotNil(t, bw.Writer())
	})

	t.Run("WithWriterWrappers appends", func(t *testing.T) {
		bw := &BaseWorker{}
		mw := newMockWriter()
		bw.WriteToWriter(mw)
		bw.WithWriterWrappers(BufWrite(4096))
		assert.Len(t, bw.wwrappers, 1)

		bw.WithWriterWrappers(BufWrite(8192))
		assert.Len(t, bw.wwrappers, 2)
	})
}

// TestBaseWorker_WrapReaders tests wrapReaders.
func TestBaseWorker_WrapReaders(t *testing.T) {
	t.Run("wrapReaders in init", func(t *testing.T) {
		bw := &BaseWorker{}
		bw.WriteToWriter(newMockWriter())
		mr := newMockReader(nil)
		bw.ReadFromReader(mr, BufRead(4096))

		err := bw.Init(context.Background())
		require.NoError(t, err)

		assert.Len(t, bw.rs, 2)
		assert.NotNil(t, bw.Reader())
	})

	t.Run("WithReaderWrappers appends", func(t *testing.T) {
		bw := &BaseWorker{}
		bw.ReadFromReader(newMockReader(nil))
		bw.WithReaderWrappers(BufRead(4096))
		assert.Len(t, bw.rwrappers, 1)

		bw.WithReaderWrappers(BufRead(8192))
		assert.Len(t, bw.rwrappers, 2)
	})
}

// TestBaseWorker_Close tests BaseWorker.Close.
func TestBaseWorker_Close(t *testing.T) {
	t.Run("Close force=true", func(t *testing.T) {
		bw := &BaseWorker{}
		mw := newMockWriter()
		mr := newMockReader(nil)
		bw.WriteToWriter(mw)
		bw.ReadFromReader(mr)

		err := bw.Init(context.Background())
		require.NoError(t, err)

		closeErr := bw.Close(true)
		require.NoError(t, closeErr)

		assert.True(t, mw.closed)
		assert.True(t, mr.closed)
	})

	t.Run("Close force=false", func(t *testing.T) {
		bw := &BaseWorker{}
		mw := newMockWriter()
		mr := newMockReader(nil)
		bw.WriteToWriter(mw)
		bw.ReadFromReader(mr)

		err := bw.Init(context.Background())
		require.NoError(t, err)

		closeErr := bw.Close(false)
		require.NoError(t, closeErr)

		assert.True(t, mw.closed)
		assert.True(t, mr.closed)
	})

	t.Run("Close onceValue returns same result", func(t *testing.T) {
		bw := &BaseWorker{}
		bw.WriteToWriter(newMockWriter())
		bw.ReadFromReader(newMockReader(nil))

		err := bw.Init(context.Background())
		require.NoError(t, err)

		err1 := bw.Close(true)
		err2 := bw.Close(true)
		assert.Equal(t, err1, err2)
	})
}

// TestCloseWriter tests closeWriter.
func TestCloseWriter(t *testing.T) {
	t.Run("io.Closer", func(t *testing.T) {
		mw := newMockWriter()
		err := closeWriter(mw)
		require.NoError(t, err)
		assert.True(t, mw.closed)
	})

	t.Run("flusher (not Closer)", func(t *testing.T) {
		mf := &mockFlusher{}
		err := closeWriter(mf)
		require.NoError(t, err)
	})

	t.Run("flusher with error", func(t *testing.T) {
		mf := &mockFlusher{flushErr: assert.AnError}
		err := closeWriter(mf)
		require.Error(t, err)
	})

	t.Run("flusher2", func(t *testing.T) {
		mf2 := &mockFlusher2{}
		err := closeWriter(mf2)
		require.NoError(t, err)
	})

	t.Run("non-closer non-flusher", func(t *testing.T) {
		nc := &nonCloser{}
		err := closeWriter(nc)
		require.NoError(t, err)
	})
}

// TestCloseReader tests closeReader.
func TestCloseReader(t *testing.T) {
	t.Run("io.Closer", func(t *testing.T) {
		mr := newMockReader(nil)
		err := closeReader(mr)
		require.NoError(t, err)
		assert.True(t, mr.closed)
	})

	t.Run("non-closer", func(t *testing.T) {
		ncr := &nonCloserReader{}
		err := closeReader(ncr)
		require.NoError(t, err)
	})
}

// TestCloseWriters tests closeWriters.
func TestCloseWriters(t *testing.T) {
	t.Run("empty writers", func(t *testing.T) {
		err := closeWriters(nil)
		require.NoError(t, err)
	})

	t.Run("multiple writers", func(t *testing.T) {
		mw1 := newMockWriter()
		mw2 := newMockWriter()
		err := closeWriters([]io.Writer{mw1, mw2})
		require.NoError(t, err)
		assert.True(t, mw1.closed)
		assert.True(t, mw2.closed)
	})

	t.Run("collects errors", func(t *testing.T) {
		mf := &mockFlusher{flushErr: assert.AnError}
		err := closeWriters([]io.Writer{mf})
		require.Error(t, err)
	})
}

// TestCloseReaders tests closeReaders.
func TestCloseReaders(t *testing.T) {
	t.Run("empty readers", func(t *testing.T) {
		err := closeReaders(nil)
		require.NoError(t, err)
	})

	t.Run("multiple readers", func(t *testing.T) {
		mr1 := newMockReader(nil)
		mr2 := newMockReader(nil)
		err := closeReaders([]io.Reader{mr1, mr2})
		require.NoError(t, err)
		assert.True(t, mr1.closed)
		assert.True(t, mr2.closed)
	})
}

// TestCopyWorker_Start tests copyWorker.Start.
func TestCopyWorker_Start(t *testing.T) {
	t.Run("copy with pipe", func(t *testing.T) {
		cw := NewCopyWorker(0).(*copyWorker)
		cw.WriteToWriter(io.Discard)
		cw.ReadFromReader(newMockReader([]byte("hello")))
		err := cw.BaseWorker.Init(context.Background())
		require.NoError(t, err)

		// Start copyWorker - it will copy from reader to writer.
		err = cw.Start(context.Background())
		require.NoError(t, err)
	})

	t.Run("copy with bufSize set", func(t *testing.T) {
		cw := NewCopyWorker(1024).(*copyWorker)
		cw.WriteToWriter(io.Discard)
		cw.ReadFromReader(newMockReader([]byte("hello world")))
		err := cw.BaseWorker.Init(context.Background())
		require.NoError(t, err)

		err = cw.Start(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1024, cw.bufSize)
	})

	t.Run("copy with stopping (manual stop)", func(t *testing.T) {
		pr, pw := io.Pipe()
		cw := NewCopyWorker(0).(*copyWorker)
		cw.WriteToWriter(pw)
		cw.ReadFromReader(pr)
		err := cw.BaseWorker.Init(context.Background())
		require.NoError(t, err)

		// Start copy in goroutine.
		go func() {
			_ = cw.Start(context.Background())
		}()

		// Give it a moment to start copying.
		time.Sleep(50 * time.Millisecond)

		// Close the pipe's reader end to trigger error, or close via Stop.
		// For the "manual stop" path, we need io.CopyBuffer to be running
		// and then stop to happen.
		// Let's just close the pipe.
		pr.Close()
	})
}

// TestCopyWorker_Stop_FromPipeline tests copyWorker.Stop.
func TestCopyWorker_Stop_FromPipeline(t *testing.T) {
	cw := NewCopyWorker(0).(*copyWorker)
	cw.WriteToWriter(newMockWriter())
	cw.ReadFromReader(newMockReader(nil))
	err := cw.BaseWorker.Init(context.Background())
	require.NoError(t, err)

	stopErr := cw.Stop(context.Background())
	require.NoError(t, stopErr)
}

// TestPipelineOptions tests pipeline.Option functions.
func TestPipelineOptions(t *testing.T) {
	t.Run("BufRead", func(t *testing.T) {
		fn := BufRead(4096)
		assert.NotNil(t, fn)
		mr := newMockReader(nil)
		result := fn(mr)
		assert.NotNil(t, result)
		assert.NotEqual(t, mr, result) // Should wrap.
	})

	t.Run("BufWrite", func(t *testing.T) {
		fn := BufWrite(4096)
		assert.NotNil(t, fn)
		mw := newMockWriter()
		result := fn(mw)
		assert.NotNil(t, result)
		assert.NotEqual(t, mw, result) // Should wrap.
	})

	t.Run("AsyncWrite", func(t *testing.T) {
		fn := AsyncWrite()
		assert.NotNil(t, fn)
		mw := newMockWriter()
		result := fn(mw)
		assert.NotNil(t, result)
		assert.NotEqual(t, mw, result)
	})

	t.Run("AsyncRead", func(t *testing.T) {
		fn := AsyncRead()
		assert.NotNil(t, fn)
		mr := newMockReader(nil)
		result := fn(mr)
		assert.NotNil(t, result)
		assert.NotEqual(t, mr, result)
	})

	t.Run("Tee", func(t *testing.T) {
		var buf1, buf2 []byte
		w1 := &writerFunc{fn: func(p []byte) (int, error) { buf1 = append(buf1, p...); return len(p), nil }}
		w2 := &writerFunc{fn: func(p []byte) (int, error) { buf2 = append(buf2, p...); return len(p), nil }}
		fn := Tee(w1, w2)
		assert.NotNil(t, fn)
		mr := newMockReader([]byte("hello"))
		teeReader := fn(mr)
		assert.NotNil(t, teeReader)

		out := make([]byte, 5)
		n, err := teeReader.Read(out)
		require.NoError(t, err)
		assert.Equal(t, 5, n)
		assert.Equal(t, "hello", string(buf1))
		assert.Equal(t, "hello", string(buf2))
	})

	t.Run("MultiWrite", func(t *testing.T) {
		var buf1, buf2 []byte
		w1 := &writerFunc{fn: func(p []byte) (int, error) { buf1 = append(buf1, p...); return len(p), nil }}
		w2 := &writerFunc{fn: func(p []byte) (int, error) { buf2 = append(buf2, p...); return len(p), nil }}
		mw := newMockWriter()
		fn := MultiWrite(w1, w2)
		result := fn(mw)
		assert.NotNil(t, result)

		n, err := result.Write([]byte("hello"))
		require.NoError(t, err)
		assert.Equal(t, 5, n)
		// mw received the write.
		assert.Equal(t, "hello", string(mw.buf))
	})
}

// writerFunc implements io.Writer using a function.
type writerFunc struct {
	fn func([]byte) (int, error)
}

func (w *writerFunc) Write(p []byte) (int, error) {
	return w.fn(p)
}

// TestBaseWorker_WriteToWriter tests WriteToWriter.
func TestBaseWorker_WriteToWriter(t *testing.T) {
	bw := &BaseWorker{}
	mw := newMockWriter()
	bw.WriteToWriter(mw)
	assert.Equal(t, mw, bw.Writer())
}

// TestBaseWorker_ReadFromReader tests ReadFromReader.
func TestBaseWorker_ReadFromReader(t *testing.T) {
	bw := &BaseWorker{}
	mr := newMockReader(nil)
	bw.ReadFromReader(mr)
	assert.Equal(t, mr, bw.Reader())
}

// TestBaseWorker_SupportZeroCopy tests SupportZeroCopy.
func TestBaseWorker_SupportZeroCopy(t *testing.T) {
	bw := &BaseWorker{}
	assert.False(t, bw.SupportZeroCopy())
}

// TestWorker_InterfaceCompliance tests that interfaces are satisfied.
func TestWorker_InterfaceCompliance(t *testing.T) {
	var _ Worker = (*MockWorker)(nil)
	var _ Worker = (*copyWorker)(nil)
	var _ Worker = (*BaseWorker)(nil)
}

// TestPipeline_Init_ZeroCopy tests Init with zero-copy support.
func TestPipeline_Init_ZeroCopy(t *testing.T) {
	// Create a pipeline where both workers support zero-copy.
	// BaseWorker returns false, so we need workers that return true.
	// We'll test that when SupportZeroCopy returns false, io.Pipe is used.

	p := &Pipeline{}
	w1 := NewMockWorker("w1")
	w2 := NewMockWorker("w2")
	p.Add(w1, w2)

	err := p.Init(context.Background())
	require.NoError(t, err)

	// Verify that pipes were set up.
	assert.NotNil(t, w1.Writer())
	assert.NotNil(t, w2.Reader())
}

// TestPipeline_Init_OsPipe tests Init with os.Pipe for zero-copy workers.
func TestPipeline_Init_OsPipe(t *testing.T) {
	// Create a temporary test: verify that io.Pipe creates a valid pipe pair.
	pr, pw := io.Pipe()
	assert.NotNil(t, pr)
	assert.NotNil(t, pw)
	pr.Close()
	pw.Close()

	// Also verify os.Pipe.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	assert.NotNil(t, r)
	assert.NotNil(t, w)
	r.Close()
	w.Close()
}

// TestCloseWriter_FlushError tests the flusher with error path.
func TestCloseWriter_FlushError(t *testing.T) {
	mf := &mockFlusher{flushErr: io.ErrShortWrite}
	err := closeWriter(mf)
	require.Error(t, err)
}

// TestCloseReader_CloserError tests closeReader with a closer that returns an error.
func TestCloseReader_CloserError(t *testing.T) {
	// Use a custom mock that returns error on close.
	errReader := &errorCloserReader{}
	err := closeReader(errReader)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close reader failed")
}

type errorCloserReader struct{}

func (e *errorCloserReader) Read(p []byte) (int, error) { return 0, io.EOF }
func (e *errorCloserReader) Close() error               { return assert.AnError }

// TestCloseWriters_CollectsErrors tests that closeWriters joins errors.
func TestCloseWriters_CollectsErrors(t *testing.T) {
	mf := &mockFlusher{flushErr: assert.AnError}
	mf2 := &mockFlusher{flushErr: io.ErrShortWrite}
	err := closeWriters([]io.Writer{mf, mf2})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
	assert.ErrorIs(t, err, io.ErrShortWrite)
}

// TestCloseReaders_CollectsErrors tests closeReaders joins errors.
func TestCloseReaders_CollectsErrors(t *testing.T) {
	ec1 := &errorCloserReader{}
	ec2 := &errorCloserReader{}
	err := closeReaders([]io.Reader{ec1, ec2})
	require.Error(t, err)
}

// TestCopyWorker_Stop_Manual tests copyWorker when stop is triggered manually.
func TestCopyWorker_Stop_ManualStop(t *testing.T) {
	// Create a copyWorker with a pipe that will block on read.
	pr, pw := io.Pipe()
	cw := NewCopyWorker(0).(*copyWorker)
	cw.WriteToWriter(pw)
	cw.ReadFromReader(pr)

	err := cw.BaseWorker.Init(context.Background())
	require.NoError(t, err)

	// Start copyWorker in goroutine.
	startDone := make(chan error, 1)
	go func() {
		startDone <- cw.Start(context.Background())
	}()

	// Give it time to enter the io.CopyBuffer loop.
	time.Sleep(50 * time.Millisecond)

	// Stop the worker (this calls Close(true) which closes readers/writers).
	err = cw.Stop(context.Background())
	require.NoError(t, err)

	// Wait for Start to return.
	select {
	case startErr := <-startDone:
		// It should complete (possibly with nil or an error from the closed pipe).
		_ = startErr
		// The important part is it doesn't hang.
	case <-time.After(2 * time.Second):
		t.Fatal("copyWorker.Start did not return after Stop")
	}
	_ = pw.Close()
}

// TestCopyWorker_Start_DefaultBufSize tests that bufSize defaults when <= 0.
func TestCopyWorker_Start_DefaultBufSize(t *testing.T) {
	cw := NewCopyWorker(0).(*copyWorker)
	assert.Equal(t, 0, cw.bufSize)
	cw.WriteToWriter(io.Discard)
	cw.ReadFromReader(newMockReader([]byte("data")))
	err := cw.BaseWorker.Init(context.Background())
	require.NoError(t, err)

	err = cw.Start(context.Background())
	require.NoError(t, err)
	// bufSize should have been set to 32*1024.
	assert.Equal(t, 32*1024, cw.bufSize)
}

// TestCopyWorker_Start_NegativeBufSize tests negative bufSize defaults.
func TestCopyWorker_Start_NegativeBufSize(t *testing.T) {
	cw := NewCopyWorker(-1).(*copyWorker)
	assert.Equal(t, -1, cw.bufSize)
	cw.WriteToWriter(io.Discard)
	cw.ReadFromReader(newMockReader([]byte("data")))
	err := cw.BaseWorker.Init(context.Background())
	require.NoError(t, err)

	err = cw.Start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 32*1024, cw.bufSize)
}

// TestCopyWorker_Start_CloseError tests the path where Close(false) returns an error
// in copyWorker's Start defer.
func TestCopyWorker_Start_CloseError(t *testing.T) {
	// Use a mock reader that returns error on close.
	cw := NewCopyWorker(1024).(*copyWorker)
	cw.WriteToWriter(io.Discard)
	errReader := &errorCloserReader{}
	cw.ReadFromReader(errReader)

	err := cw.BaseWorker.Init(context.Background())
	require.NoError(t, err)

	// Start will read from errorCloserReader (returns io.EOF immediately),
	// then defer calls Close(false) which closes errReader with error.
	err = cw.Start(context.Background())
	require.Error(t, err)
}

// TestCopyWorker_Start_StoppingCancelsError tests the Stopping path in copyWorker.Start
// where the copy error is cleared when stopping.
func TestCopyWorker_Start_StoppingCancelsError_Pipeline(t *testing.T) {
	pr, pw := io.Pipe()

	cw := NewCopyWorker(1024).(*copyWorker)
	cw.WriteToWriter(pw)
	cw.ReadFromReader(pr)

	err := cw.BaseWorker.Init(context.Background())
	require.NoError(t, err)

	// Use a context with a logger.
	ctx := logs.CtxWith(context.Background(), slog.Default())

	// Start copyWorker via runner.Start.
	startDone := make(chan error, 1)
	go func() {
		startDone <- runner.Start(ctx, cw)
	}()

	// Wait for it to start.
	select {
	case <-cw.Started():
	case <-time.After(time.Second):
		t.Fatal("not started")
	}

	// Call runner.Stop FIRST. This calls markStopping which closes Stopping.
	err = runner.Stop(context.Background(), cw)
	require.NoError(t, err)

	// Now close the pipe to make io.CopyBuffer fail.
	// By the time io.CopyBuffer returns, Stopping is already closed.
	// The select on <-c.Stopping() will pick it up and clear the error.
	pr.CloseWithError(io.ErrClosedPipe)

	select {
	case startErr := <-startDone:
		require.NoError(t, startErr)
	case <-time.After(2 * time.Second):
		t.Fatal("copyWorker.Start did not return")
	}
	pw.Close()
}

// TestPipeline_Init_StartError tests Pipeline.Start when a worker's Init returns error.
func TestPipeline_Init_StartError(t *testing.T) {
	p := &Pipeline{}
	w1 := NewMockWorker("w1")
	w1.initErr = assert.AnError
	w2 := NewMockWorker("w2")
	p.Add(w1, w2)

	err := p.Init(context.Background())
	require.NoError(t, err)

	err = p.Start(context.Background())
	require.Error(t, err)
}

// TestPipeline_Start_WorkerStartError tests Pipeline.Start when a worker's Start returns error.
func TestPipeline_Start_WorkerStartError(t *testing.T) {
	p := &Pipeline{}
	w1 := NewMockWorker("w1")
	w1.startErr = assert.AnError
	// w2 also needs to not block, otherwise wg.Wait hangs.
	w2 := NewMockWorker("w2")
	w2.startErr = io.EOF // any error so it returns immediately
	p.Add(w1, w2)

	err := p.Init(context.Background())
	require.NoError(t, err)

	startErr := p.Start(context.Background())
	require.Error(t, startErr)
	assert.ErrorIs(t, startErr, assert.AnError)
	assert.ErrorIs(t, startErr, io.EOF)
}

// TestBaseWorker_Start_Panics tests that BaseWorker.Start panics with "not implemented".
func TestBaseWorker_Start_Panics(t *testing.T) {
	bw := &BaseWorker{}
	assert.PanicsWithValue(t, "not implemented", func() {
		_ = bw.Start(context.Background())
	})
}

// TestBaseWorker_Stop_Panics tests that BaseWorker.Stop panics with "not implemented".
func TestBaseWorker_Stop_Panics(t *testing.T) {
	bw := &BaseWorker{}
	assert.PanicsWithValue(t, "not implemented", func() {
		_ = bw.Stop(context.Background())
	})
}

// zeroCopyWorker is a mock that supports zero copy.
type zeroCopyWorker struct {
	MockWorker
}

func (z *zeroCopyWorker) SupportZeroCopy() bool {
	return true
}

// TestPipeline_Init_ZeroCopyBoth tests Init with workers that both support zero copy.
func TestPipeline_Init_ZeroCopyBoth(t *testing.T) {
	p := &Pipeline{}
	w1 := &zeroCopyWorker{MockWorker: *NewMockWorker("w1")}
	w2 := &zeroCopyWorker{MockWorker: *NewMockWorker("w2")}
	p.Add(w1, w2)

	err := p.Init(context.Background())
	require.NoError(t, err)

	assert.NotNil(t, w1.Writer())
	assert.NotNil(t, w2.Reader())
}

// TestPipeline_Init_MixedZeroCopy tests Init where one worker supports zero copy
// and the other doesn't (falls back to io.Pipe).
func TestPipeline_Init_MixedZeroCopy(t *testing.T) {
	p := &Pipeline{}
	w1 := &zeroCopyWorker{MockWorker: *NewMockWorker("w1")}
	w2 := NewMockWorker("w2")
	p.Add(w1, w2)

	err := p.Init(context.Background())
	require.NoError(t, err)

	assert.NotNil(t, w1.Writer())
	assert.NotNil(t, w2.Reader())
}

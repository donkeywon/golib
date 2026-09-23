package pipeline

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---- 测试替身：writer 形态覆盖 closeWriter 的所有分支 ----

type plainWriter struct{ n int }

func (w *plainWriter) Write(p []byte) (int, error) { w.n += len(p); return len(p), nil }

type closerWriter struct {
	closed bool
	err    error
	panic  bool
}

func (w *closerWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *closerWriter) Close() error {
	if w.panic {
		panic("close writer panic")
	}
	w.closed = true
	return w.err
}

type flusherWriter struct {
	flushed bool
	err     error
}

func (w *flusherWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *flusherWriter) Flush() error {
	w.flushed = true
	return w.err
}

type flusherNoErrWriter struct{ flushed bool }

func (w *flusherNoErrWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *flusherNoErrWriter) Flush()                      { w.flushed = true }

// ---- 测试替身：reader 形态 ----

type plainReader struct{ s string }

func (r *plainReader) Read(p []byte) (int, error) {
	if r.s == "" {
		return 0, io.EOF
	}
	n := copy(p, r.s)
	r.s = r.s[n:]
	return n, nil
}

type closerReader struct {
	closed bool
	err    error
	panic  bool
}

func (r *closerReader) Read([]byte) (int, error) { return 0, io.EOF }
func (r *closerReader) Close() error {
	if r.panic {
		panic("close reader panic")
	}
	r.closed = true
	return r.err
}

// ---- closeWriter / closeReaders ----

func TestCloseWriter(t *testing.T) {
	t.Run("closer ok", func(t *testing.T) {
		w := &closerWriter{}
		require.NoError(t, closeWriter(w))
		require.True(t, w.closed)
	})

	t.Run("closer error", func(t *testing.T) {
		w := &closerWriter{err: errors.New("close boom")}
		err := closeWriter(w)
		require.ErrorContains(t, err, "close writer failed")
	})

	t.Run("closer panic", func(t *testing.T) {
		w := &closerWriter{panic: true}
		err := closeWriter(w)
		require.ErrorContains(t, err, "panic on close writer")
	})

	t.Run("flusher ok", func(t *testing.T) {
		w := &flusherWriter{}
		require.NoError(t, closeWriter(w))
		require.True(t, w.flushed)
	})

	t.Run("flusher error", func(t *testing.T) {
		w := &flusherWriter{err: errors.New("flush boom")}
		err := closeWriter(w)
		require.ErrorContains(t, err, "flush writer failed")
	})

	t.Run("flusher2", func(t *testing.T) {
		w := &flusherNoErrWriter{}
		require.NoError(t, closeWriter(w))
		require.True(t, w.flushed)
	})

	t.Run("plain writer", func(t *testing.T) {
		require.NoError(t, closeWriter(&plainWriter{}))
	})
}

func TestCloseWriters(t *testing.T) {
	require.NoError(t, closeWriters(nil))

	// 混合：无操作 + 出错 -> 聚合错误
	w1 := &plainWriter{}
	w2 := &closerWriter{err: errors.New("boom")}
	err := closeWriters([]io.Writer{w1, w2, nil})
	require.ErrorContains(t, err, "close writer failed")
	require.True(t, w2.closed)
}

func TestCloseReader(t *testing.T) {
	t.Run("not closer", func(t *testing.T) {
		require.NoError(t, closeReader(&plainReader{s: "x"}))
	})

	t.Run("closer ok", func(t *testing.T) {
		r := &closerReader{}
		require.NoError(t, closeReader(r))
		require.True(t, r.closed)
	})

	t.Run("closer error", func(t *testing.T) {
		err := closeReader(&closerReader{err: errors.New("close boom")})
		require.ErrorContains(t, err, "close reader failed")
	})

	t.Run("closer panic", func(t *testing.T) {
		err := closeReader(&closerReader{panic: true})
		require.ErrorContains(t, err, "panic on close reader")
	})
}

func TestCloseReaders(t *testing.T) {
	require.NoError(t, closeReaders(nil))

	r1 := &plainReader{s: "x"}
	r2 := &closerReader{err: errors.New("boom")}
	err := closeReaders([]io.Reader{r1, r2, nil})
	require.ErrorContains(t, err, "close reader failed")
	require.True(t, r2.closed)
}

// ---- BaseWorker 包装与访问器 ----

// 未接线时 Init：wrapWriters/wrapReaders 直接返回（单次 Init，符合 Init 只调用一次的前提）。
func TestBaseWorkerWrappersNilEndpoints(t *testing.T) {
	wk := &BaseWorker{}
	require.NoError(t, wk.Init(context.Background()))

	require.Nil(t, wk.Reader())
	require.Nil(t, wk.Writer())
	require.Nil(t, wk.rs)
	require.Nil(t, wk.ws)
}

// 接线 + wrapper 后 Init：按注册顺序套用（单次 Init）。
func TestBaseWorkerWrappersChain(t *testing.T) {
	wk := &BaseWorker{}
	inner := &plainWriter{}
	reader := &plainReader{s: "hello"}
	wk.WriteToWriter(inner)
	wk.ReadFromReader(reader)

	require.False(t, wk.WriterWrapped())
	require.False(t, wk.ReaderWrapped())
	require.False(t, wk.SupportZeroCopy())

	wk.WithWriterWrappers(BufWrite(8), MultiWrite(&bytes.Buffer{}))
	wk.WithReaderWrappers(Tee(&bytes.Buffer{}))
	require.True(t, wk.WriterWrapped())
	require.True(t, wk.ReaderWrapped())

	require.NoError(t, wk.Init(context.Background()))

	// ws = [inner, bufio(inner), multi(...)]，rs = [reader, tee(reader)]
	require.Len(t, wk.ws, 3)
	require.Len(t, wk.rs, 2)
	require.IsType(t, &plainWriter{}, wk.ws[0])
	require.Equal(t, inner, wk.ws[0])
	require.NotSame(t, inner, wk.Writer())
}

func TestBaseWorkerCloseOrder(t *testing.T) {
	var order []string
	mk := func(name string) io.Writer {
		return &orderWriter{name: name, order: &order}
	}

	wk := &BaseWorker{}
	wk.WriteToWriter(mk("sink"))
	wk.WithWriterWrappers(func(io.Writer) io.Writer { return mk("w1") },
		func(io.Writer) io.Writer { return mk("w2") })
	require.NoError(t, wk.Init(context.Background()))

	// 优雅关闭：逆序（外层先关）
	require.NoError(t, wk.Close(false))
	require.Equal(t, []string{"w2", "w1", "sink"}, order)

	// 强制关闭：正序（内层先关）
	order = nil
	require.NoError(t, wk.Close(true))
	require.Equal(t, []string{"sink", "w1", "w2"}, order)
}

func TestBaseWorkerCloseAggregatesErrors(t *testing.T) {
	wk := &BaseWorker{}
	wk.WriteToWriter(&closerWriter{err: errors.New("w boom")})
	wk.ReadFromReader(&closerReader{err: errors.New("r boom")})
	require.NoError(t, wk.Init(context.Background()))

	err := wk.Close(false)
	require.ErrorContains(t, err, "close writer failed")
	require.ErrorContains(t, err, "close reader failed")
}

func TestBaseWorkerCloseWithoutInit(t *testing.T) {
	// 未 Init 就 Close：closeOnce/forceCloseOnce 为 nil -> panic（缺陷，见报告）
	wk := &BaseWorker{}
	require.Panics(t, func() { _ = wk.Close(false) })
	require.Panics(t, func() { _ = wk.Close(true) })
}

func TestBaseWorkerRunNotImplemented(t *testing.T) {
	require.PanicsWithValue(t, "not implemented", func() { _ = (&BaseWorker{}).Run(context.Background()) })
}

type orderWriter struct {
	name  string
	order *[]string
	n     int
}

func (w *orderWriter) Write(p []byte) (int, error) {
	w.n += len(p)
	return len(p), nil
}
func (w *orderWriter) Close() error {
	*w.order = append(*w.order, w.name)
	return nil
}

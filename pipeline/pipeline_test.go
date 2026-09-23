package pipeline

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ---- 测试用 worker ----

// fakeWorker 实现 Worker（通过 BaseWorker 获得 Init）。
type fakeWorker struct{ BaseWorker }

func (w *fakeWorker) Run(context.Context) error { return nil }

// zeroCopyWorker 声明支持零拷贝（用于触发 [os.Pipe] 分支）。
type zeroCopyWorker struct{ fakeWorker }

func (w *zeroCopyWorker) SupportZeroCopy() bool { return true }

// failInitWorker 的 Init 失败（用于 Run 的 init 错误分支）。
type failInitWorker struct{ BaseWorker }

func (w *failInitWorker) Init(context.Context) error { return errors.New("init boom") }
func (w *failInitWorker) Run(context.Context) error  { return nil }

// errWorker 的 Run 返回错误。
type errWorker struct{ fakeWorker }

func (w *errWorker) Run(context.Context) error { return errors.New("run boom") }

// panicWorker 的 Run panic。
type panicWorker struct{ fakeWorker }

func (w *panicWorker) Run(context.Context) error { panic("run panic") }

// noInitWorker 不实现 initializer，用于覆盖 Run 里的类型断言失败分支。
type noInitWorker struct {
	w io.Writer
	r io.Reader
}

func (w *noInitWorker) Run(context.Context) error                       { return nil }
func (w *noInitWorker) Writer() io.Writer                               { return w.w }
func (w *noInitWorker) Reader() io.Reader                               { return w.r }
func (w *noInitWorker) WriteToWriter(x io.Writer, _ ...WriterWrapFunc)  { w.w = x }
func (w *noInitWorker) ReadFromReader(x io.Reader, _ ...ReaderWrapFunc) { w.r = x }
func (w *noInitWorker) WithWriterWrappers(...WriterWrapFunc)            {}
func (w *noInitWorker) WithReaderWrappers(...ReaderWrapFunc)            {}
func (w *noInitWorker) WriterWrapped() bool                             { return false }
func (w *noInitWorker) ReaderWrapped() bool                             { return false }
func (w *noInitWorker) SupportZeroCopy() bool                           { return false }

// ---- Pipeline 基础 ----

func TestNewAndAdd(t *testing.T) {
	p := New()
	require.NotNil(t, p)
	require.Same(t, p, p.Add(&fakeWorker{}))
	p.Add(&fakeWorker{}, &fakeWorker{})
	require.Len(t, p.ws, 3)
}

func TestPipelineInitNoWorkers(t *testing.T) {
	require.PanicsWithValue(t, "no workers", func() { _ = New().Init(context.Background()) })
}

func TestPipelineInitWriterAlreadyExists(t *testing.T) {
	w0 := &fakeWorker{}
	w0.WriteToWriter(&plainWriter{})
	require.PanicsWithValue(t,
		"writer or reader already exists between workers: *pipeline.fakeWorker(0) and *pipeline.fakeWorker(1)",
		func() { _ = New().Add(w0, &fakeWorker{}).Init(context.Background()) })
}

func TestPipelineInitReaderAlreadyExists(t *testing.T) {
	w1 := &fakeWorker{}
	w1.ReadFromReader(&plainReader{s: "x"})
	require.PanicsWithValue(t,
		"writer or reader already exists between workers: *pipeline.fakeWorker(0) and *pipeline.fakeWorker(1)",
		func() { _ = New().Add(&fakeWorker{}, w1).Init(context.Background()) })
}

func TestPipelineInitSingleWorker(t *testing.T) {
	// len(ws) == 1：不改写任何 reader/writer
	w := &fakeWorker{}
	w.WriteToWriter(&plainWriter{})
	p := New().Add(w)
	require.NoError(t, p.Init(context.Background()))
	require.Empty(t, p.osPipes)
	require.NotNil(t, w.Writer())
}

func TestPipelineInitIOPipe(t *testing.T) {
	w0, w1 := &fakeWorker{}, &fakeWorker{}
	p := New().Add(w0, w1)
	require.NoError(t, p.Init(context.Background()))
	require.Empty(t, p.osPipes, "非零拷贝应使用 io.Pipe")

	require.NotNil(t, w0.Writer())
	require.NotNil(t, w1.Reader())
	// 两端是同一个 io.Pipe 的两半：写入一端可从另一端读出
	_, ok := w0.Writer().(*io.PipeWriter)
	require.True(t, ok)
	_, ok = w1.Reader().(*io.PipeReader)
	require.True(t, ok)
}

func TestPipelineInitZeroCopyPipe(t *testing.T) {
	w0, w1 := &zeroCopyWorker{}, &zeroCopyWorker{}
	p := New().Add(w0, w1)
	require.NoError(t, p.Init(context.Background()))
	require.Len(t, p.osPipes, 2, "零拷贝应使用 os.Pipe")

	require.IsType(t, &os.File{}, w0.Writer())
	require.IsType(t, &os.File{}, w1.Reader())

	require.NoError(t, p.closeOSPipes())
	// 注意：closeOSPipes 不幂等，第二次调用会返回 "file already closed"（仅错误路径调用，影响有限）
	require.Error(t, p.closeOSPipes())
}

func TestPipelineInitWrappedDisablesZeroCopy(t *testing.T) {
	w0, w1 := &zeroCopyWorker{}, &zeroCopyWorker{}
	w0.WithWriterWrappers(BufWrite(4)) // 有 wrapper -> 不能用 os.Pipe
	p := New().Add(w0, w1)
	require.NoError(t, p.Init(context.Background()))
	require.Empty(t, p.osPipes)
}

func TestPipelineInitWrappedReaderDisablesZeroCopy(t *testing.T) {
	w0, w1 := &zeroCopyWorker{}, &zeroCopyWorker{}
	w1.WithReaderWrappers(BufRead(4)) // 上游无 wrapper、下游有 -> 仍不能用 os.Pipe
	p := New().Add(w0, w1)
	require.NoError(t, p.Init(context.Background()))
	require.Empty(t, p.osPipes)
}

func TestCloseOSPipesEmpty(t *testing.T) {
	require.NoError(t, New().closeOSPipes())
}

// 通过压低 RLIMIT_NOFILE 让 [os.Pipe] 失败，覆盖 Init 的 [os.Pipe] 错误分支。
func TestPipelineInitOSPipeError(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("rlimit 语义仅 linux 稳定")
	}

	var orig syscall.Rlimit
	require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_NOFILE, &orig))
	low := orig
	low.Cur = 32

	defer func() { _ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &orig) }()
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_NOFILE, &low))

	var held []*os.File
	defer func() {
		for _, f := range held {
			_ = f.Close()
		}
	}()
	for {
		f, err := os.Open("/dev/null")
		if err != nil {
			break
		}
		held = append(held, f)
	}

	p := New().Add(&zeroCopyWorker{}, &zeroCopyWorker{})
	err := p.Init(context.Background())

	require.ErrorContains(t, err, "create os pipe failed")
	require.Empty(t, p.osPipes)
}

// ---- Pipeline.Run ----

func TestPipelineRunEndToEnd(t *testing.T) {
	var out bytes.Buffer
	w0 := NewCopyWorker(16).(*copyWorker)
	w0.ReadFromReader(strings.NewReader("hello pipeline"))
	w1 := NewCopyWorker(16).(*copyWorker)
	w1.WriteToWriter(&out)

	p := New().Add(w0, w1)
	require.NoError(t, p.Init(context.Background()))
	require.NoError(t, p.Run(context.Background()))
	require.Equal(t, "hello pipeline", out.String())
}

func TestPipelineRunWorkerWithoutInit(t *testing.T) {
	// 覆盖 initializer 类型断言失败分支
	p := New().Add(&noInitWorker{})
	require.NoError(t, p.Init(context.Background()))
	require.NoError(t, p.Run(context.Background()))
}

func TestPipelineRunWorkerError(t *testing.T) {
	p := New().Add(&errWorker{})
	require.NoError(t, p.Init(context.Background()))
	err := p.Run(context.Background())
	require.ErrorContains(t, err, "pipeline worker failed")
	require.ErrorContains(t, err, "run boom")
}

func TestPipelineRunWorkerPanic(t *testing.T) {
	p := New().Add(&panicWorker{}, &errWorker{})
	require.NoError(t, p.Init(context.Background()))
	err := p.Run(context.Background())
	require.ErrorContains(t, err, "panic on run worker 0")
	require.ErrorContains(t, err, "run panic")
	require.ErrorContains(t, err, "pipeline worker failed: *pipeline.errWorker(1)")
}

func TestPipelineRunWorkerInitError(t *testing.T) {
	// 零拷贝对产生 os.Pipe；最后一个 worker Init 失败 -> 走 closeOSPipes
	p := New().Add(&zeroCopyWorker{}, &zeroCopyWorker{}, &failInitWorker{})
	require.NoError(t, p.Init(context.Background()))
	require.Len(t, p.osPipes, 2)

	err := p.Run(context.Background())
	require.ErrorContains(t, err, "init worker failed")
	require.ErrorContains(t, err, "init boom")
	require.NotContains(t, err.Error(), "close os pipes failed")
}

func TestPipelineRunWorkerInitErrorClosePipeFails(t *testing.T) {
	p := New().Add(&zeroCopyWorker{}, &zeroCopyWorker{}, &failInitWorker{})
	require.NoError(t, p.Init(context.Background()))
	require.NoError(t, p.osPipes[0].Close()) // 先关掉一个 -> 兜底关闭时产生错误

	err := p.Run(context.Background())
	require.ErrorContains(t, err, "init worker failed")
	require.ErrorContains(t, err, "close os pipes failed")
}

func TestPipelineRunWorkerInitErrorNoOSPipes(t *testing.T) {
	// closeOSPipes 的 len==0 分支
	p := New().Add(&failInitWorker{})
	require.NoError(t, p.Init(context.Background()))
	err := p.Run(context.Background())
	require.ErrorContains(t, err, "init worker failed")
	require.Empty(t, p.osPipes)
}

func TestPipelineRunAndWaitForWorkerFinish(t *testing.T) {
	// 确保 Run 会等待所有 worker 返回
	done := make(chan struct{})
	slow := &slowWorker{delay: 50 * time.Millisecond, done: done}
	p := New().Add(slow)
	require.NoError(t, p.Init(context.Background()))
	require.NoError(t, p.Run(context.Background()))
	select {
	case <-done:
	default:
		t.Fatal("Run 返回时 worker 尚未结束")
	}
}

type slowWorker struct {
	fakeWorker

	delay time.Duration
	done  chan struct{}
}

func (w *slowWorker) Run(_ context.Context) error {
	time.Sleep(w.delay)
	close(w.done)
	return nil
}

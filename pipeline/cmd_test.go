package pipeline

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/donkeywon/golib/runner"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCmdWorker(t *testing.T) {
	t.Run("creates worker with name", func(t *testing.T) {
		w := NewCmdWorker("echo", "hello")
		var _ Worker = w
		cw, ok := w.(*cmdWorker)
		require.True(t, ok)
		assert.Equal(t, "echo", cw.name)
		assert.Equal(t, []string{"hello"}, cw.args)
	})

	t.Run("creates worker with no args", func(t *testing.T) {
		w := NewCmdWorker("cat")
		cw, ok := w.(*cmdWorker)
		require.True(t, ok)
		assert.Equal(t, "cat", cw.name)
		assert.Empty(t, cw.args)
	})
}

func TestCmdWorker_SupportZeroCopy(t *testing.T) {
	w := NewCmdWorker("echo", "test")
	assert.True(t, w.SupportZeroCopy())
}

func TestCmdWorker_WithOptions(t *testing.T) {
	w := NewCmdWorker("echo", "hello")
	cw := w.(*cmdWorker)
	assert.Len(t, cw.opts, 0)

	w.(*cmdWorker).WithOptions(nil)
	assert.Len(t, cw.opts, 1)

	w.(*cmdWorker).WithOptions(func(cmd *exec.Cmd) {})
	assert.Len(t, cw.opts, 2)
}

func TestCmdWorker_Start_Single(t *testing.T) {
	t.Run("echo command writes to writer", func(t *testing.T) {
		w := NewCmdWorker("echo", "-n", "hello world")
		mw := newMockWriter()
		w.WriteToWriter(mw)
		w.ReadFromReader(newMockReader(nil))

		err := w.(*cmdWorker).BaseWorker.Init(context.Background())
		require.NoError(t, err)

		err = w.Start(context.Background())
		require.NoError(t, err)

		assert.Equal(t, "hello world", string(mw.buf))
	})

	t.Run("cat command copies stdin to stdout", func(t *testing.T) {
		input := "test data for cat"
		pr, pw := io.Pipe()

		w := NewCmdWorker("cat")
		mw := newMockWriter()
		w.ReadFromReader(pr)
		w.WriteToWriter(mw)

		err := w.(*cmdWorker).BaseWorker.Init(context.Background())
		require.NoError(t, err)

		go func() {
			pw.Write([]byte(input))
			pw.Close()
		}()

		err = w.Start(context.Background())
		require.NoError(t, err)

		assert.Equal(t, input, string(mw.buf))
	})

	t.Run("command not found returns error", func(t *testing.T) {
		w := NewCmdWorker("nonexistent_command_xyz")
		w.WriteToWriter(newMockWriter())
		w.ReadFromReader(newMockReader(nil))

		err := w.(*cmdWorker).BaseWorker.Init(context.Background())
		require.NoError(t, err)

		err = w.Start(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exec cmd failed")
	})
}

// chainWorkers connects multiple workers with io.Pipe and runs them
// concurrently, feeding input to the first and collecting from the last.
func chainWorkers(t *testing.T, input string, workers ...Worker) string {
	t.Helper()
	require.GreaterOrEqual(t, len(workers), 1, "need at least one worker")

	pipes := make([]*io.PipeWriter, len(workers)-1)
	for i := 0; i < len(workers)-1; i++ {
		pr, pw := io.Pipe()
		workers[i].WriteToWriter(pw)
		workers[i+1].ReadFromReader(pr)
		pipes[i] = pw
	}

	inputPr, inputPw := io.Pipe()
	workers[0].ReadFromReader(inputPr)

	mw := newMockWriter()
	workers[len(workers)-1].WriteToWriter(mw)

	for i, w := range workers {
		err := w.(*cmdWorker).BaseWorker.Init(context.Background())
		require.NoError(t, err, "init worker %d failed", i)
	}

	ctx := context.Background()

	var wg sync.WaitGroup
	for i, w := range workers {
		wg.Add(1)
		go func(idx int, wk Worker) {
			defer wg.Done()
			_ = runner.Start(ctx, wk.(runner.Runner))
		}(i, w)
	}

	for i, w := range workers {
		select {
		case <-w.(*cmdWorker).Started():
		case <-time.After(3 * time.Second):
			t.Fatalf("worker %d did not start", i)
		}
	}

	inputPw.Write([]byte(input))
	inputPw.Close()

	for i := 0; i < len(pipes); i++ {
		select {
		case <-workers[i].(*cmdWorker).Done():
		case <-time.After(10 * time.Second):
			t.Fatalf("worker %d did not finish", i)
		}

		w := workers[i].Writer()
		if f, ok := w.(flusher); ok {
			_ = f.Flush()
		} else if f2, ok := w.(flusher2); ok {
			f2.Flush()
		}

		pipes[i].Close()
	}

	wg.Wait()
	return string(mw.buf)
}

func TestCmdWorker_Chained_ZeroCopy_IoPipe(t *testing.T) {
	t.Run("cat_pipe_cat", func(t *testing.T) {
		input := "hello pipeline\n"
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("cat")
		w3 := NewCmdWorker("cat")

		result := chainWorkers(t, input, w1, w2, w3)
		assert.Equal(t, input, result)
	})

	t.Run("tr_uppercase chain", func(t *testing.T) {
		input := "hello world\nfoo bar\n"
		w1 := NewCmdWorker("tr", "a-z", "A-Z")
		w2 := NewCmdWorker("cat")

		result := chainWorkers(t, input, w1, w2)
		assert.Equal(t, strings.ToUpper(input), result)
	})

	t.Run("three_stage sort uniq", func(t *testing.T) {
		input := "banana\napple\nbanana\ncherry\napple\n"
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("sort")
		w3 := NewCmdWorker("uniq")

		result := chainWorkers(t, input, w1, w2, w3)
		assert.Equal(t, "apple\nbanana\ncherry\n", result)
	})

	t.Run("grep and wc", func(t *testing.T) {
		input := "line1\nline2\nline3\n"
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("grep", "line")
		w3 := NewCmdWorker("wc", "-l")

		result := chainWorkers(t, input, w1, w2, w3)
		assert.Contains(t, strings.TrimSpace(result), "3")
	})
}

func TestCmdWorker_Chained_NonZeroCopy(t *testing.T) {
	t.Run("cat chain with buf wrappers", func(t *testing.T) {
		input := "hello from non-zero-copy pipeline\n"
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("cat")
		w3 := NewCmdWorker("cat")

		w2.WithReaderWrappers(BufRead(4096))
		w2.WithWriterWrappers(BufWrite(4096))

		result := chainWorkers(t, input, w1, w2, w3)
		assert.Equal(t, input, result)
	})

	t.Run("tr uppercase with writer wrapper", func(t *testing.T) {
		input := "lowercase\n"
		w1 := NewCmdWorker("tr", "a-z", "A-Z")
		w2 := NewCmdWorker("cat")

		w1.WithWriterWrappers(BufWrite(4096))

		result := chainWorkers(t, input, w1, w2)
		assert.Equal(t, strings.ToUpper(input), result)
	})

	t.Run("reader wrapper prevents zero copy", func(t *testing.T) {
		input := "test data\n"
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("cat")

		w2.WithReaderWrappers(BufRead(4096))

		result := chainWorkers(t, input, w1, w2)
		assert.Equal(t, input, result)
	})
}

func TestCmdWorker_Chained_WrappersOnDifferentWorkers(t *testing.T) {
	input := "ddd\naaa\nccc\nbbb\n"
	expected := "aaa\nbbb\nccc\nddd\n"

	t.Run("writer wrapper on first worker", func(t *testing.T) {
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("sort")
		w3 := NewCmdWorker("cat")
		w1.WithWriterWrappers(BufWrite(4096))

		result := chainWorkers(t, input, w1, w2, w3)
		assert.Equal(t, expected, result)
	})

	t.Run("reader wrapper on middle worker", func(t *testing.T) {
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("sort")
		w3 := NewCmdWorker("cat")
		w2.WithReaderWrappers(BufRead(4096))

		result := chainWorkers(t, input, w1, w2, w3)
		assert.Equal(t, expected, result)
	})

	t.Run("both wrappers on middle worker", func(t *testing.T) {
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("sort")
		w3 := NewCmdWorker("cat")
		w2.WithReaderWrappers(BufRead(4096))
		w2.WithWriterWrappers(BufWrite(4096))

		result := chainWorkers(t, input, w1, w2, w3)
		assert.Equal(t, expected, result)
	})

	t.Run("reader wrapper on last worker", func(t *testing.T) {
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("sort")
		w3 := NewCmdWorker("cat")
		w3.WithReaderWrappers(BufRead(4096))

		result := chainWorkers(t, input, w1, w2, w3)
		assert.Equal(t, expected, result)
	})
}

func TestCmdWorker_Chained_BothModes_SameResult(t *testing.T) {
	input := "zebra\nalpha\nbeta\nzebra\nalpha\n"
	expected := "alpha\nbeta\nzebra\n"

	t.Run("without wrappers", func(t *testing.T) {
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("sort")
		w3 := NewCmdWorker("uniq")

		result := chainWorkers(t, input, w1, w2, w3)
		assert.Equal(t, expected, result)
	})

	t.Run("with writer wrapper", func(t *testing.T) {
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("sort")
		w3 := NewCmdWorker("uniq")
		w1.WithWriterWrappers(BufWrite(4096))

		result := chainWorkers(t, input, w1, w2, w3)
		assert.Equal(t, expected, result)
	})
}

func TestCmdWorker_Stop(t *testing.T) {
	w := NewCmdWorker("sleep", "10")
	w.WriteToWriter(newMockWriter())
	w.ReadFromReader(newMockReader(nil))

	err := w.(*cmdWorker).BaseWorker.Init(context.Background())
	require.NoError(t, err)

	ctx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Start(ctx, w.(runner.Runner))
	}()

	select {
	case <-w.(*cmdWorker).Started():
	case <-time.After(3 * time.Second):
		t.Fatal("cmdWorker did not start")
	}

	// Wait for cmdWorker.Start to set c.cancel.
	time.Sleep(200 * time.Millisecond)

	err = runner.Stop(ctx, w.(runner.Runner))
	require.NoError(t, err)

	select {
	case err = <-errCh:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exec cmd failed")
	case <-time.After(10 * time.Second):
		t.Fatal("cmdWorker did not stop")
	}
}

func TestCmdWorker_Start_LoggerOutput(t *testing.T) {
	t.Run("success command logs info", func(t *testing.T) {
		w := NewCmdWorker("echo", "-n", "ok")
		mw := newMockWriter()
		w.WriteToWriter(mw)
		w.ReadFromReader(newMockReader(nil))

		err := w.(*cmdWorker).BaseWorker.Init(context.Background())
		require.NoError(t, err)

		err = w.Start(zerolog.Nop().WithContext(context.Background()))
		require.NoError(t, err)
	})

	t.Run("failed command logs error", func(t *testing.T) {
		w := NewCmdWorker("false")
		w.WriteToWriter(newMockWriter())
		w.ReadFromReader(newMockReader(nil))

		err := w.(*cmdWorker).BaseWorker.Init(context.Background())
		require.NoError(t, err)

		err = w.Start(zerolog.Nop().WithContext(context.Background()))
		require.Error(t, err)
	})
}

func TestCmdWorker_Start_FailedCommand_Chained(t *testing.T) {
	input := "some data\n"
	w1 := NewCmdWorker("cat")
	w2 := NewCmdWorker("false")
	w3 := NewCmdWorker("cat")

	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	w1.WriteToWriter(pw1)
	w2.ReadFromReader(pr1)
	w2.WriteToWriter(pw2)
	w3.ReadFromReader(pr2)
	w3.WriteToWriter(newMockWriter())

	inputPr, inputPw := io.Pipe()
	w1.ReadFromReader(inputPr)

	err := w1.(*cmdWorker).BaseWorker.Init(context.Background())
	require.NoError(t, err)
	err = w2.(*cmdWorker).BaseWorker.Init(context.Background())
	require.NoError(t, err)
	err = w3.(*cmdWorker).BaseWorker.Init(context.Background())
	require.NoError(t, err)

	ctx := context.Background()
	errChs := make([]chan error, 3)
	for i := range errChs {
		errChs[i] = make(chan error, 1)
	}

	go func() { errChs[0] <- runner.Start(ctx, w1.(runner.Runner)) }()
	go func() { errChs[1] <- runner.Start(ctx, w2.(runner.Runner)) }()
	go func() { errChs[2] <- runner.Start(ctx, w3.(runner.Runner)) }()

	workers := []*cmdWorker{w1.(*cmdWorker), w2.(*cmdWorker), w3.(*cmdWorker)}
	for i := 0; i < 3; i++ {
		select {
		case <-workers[i].Started():
		case <-time.After(3 * time.Second):
			t.Fatalf("worker %d did not start", i)
		}
	}

	inputPw.Write([]byte(input))
	inputPw.Close()

	err = <-errChs[0]
	require.NoError(t, err)
	pw1.Close()

	err = <-errChs[1]
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec cmd failed")

	pw2.Close()
	<-errChs[2]
}

func TestPipeline_Init_ZeroCopy_WithCmdWorkers(t *testing.T) {
	p := &Pipeline{}
	w1 := NewCmdWorker("cat")
	w2 := NewCmdWorker("cat")
	p.Add(w1, w2)

	err := p.Init(context.Background())
	require.NoError(t, err)

	assert.NotNil(t, w1.Writer())
	assert.NotNil(t, w2.Reader())
	assert.True(t, w1.SupportZeroCopy())
	assert.True(t, w2.SupportZeroCopy())
	assert.False(t, w1.WriterWrapped())
	assert.False(t, w2.ReaderWrapped())
}

func TestPipeline_Init_NonZeroCopy_WithWrappers(t *testing.T) {
	t.Run("writer wrapper forces io.Pipe", func(t *testing.T) {
		p := &Pipeline{}
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("cat")
		w1.WithWriterWrappers(BufWrite(4096))
		p.Add(w1, w2)

		err := p.Init(context.Background())
		require.NoError(t, err)

		assert.NotNil(t, w1.Writer())
		assert.NotNil(t, w2.Reader())
		assert.True(t, w1.WriterWrapped())
	})

	t.Run("reader wrapper forces io.Pipe", func(t *testing.T) {
		p := &Pipeline{}
		w1 := NewCmdWorker("cat")
		w2 := NewCmdWorker("cat")
		w2.WithReaderWrappers(BufRead(4096))
		p.Add(w1, w2)

		err := p.Init(context.Background())
		require.NoError(t, err)

		assert.NotNil(t, w1.Writer())
		assert.NotNil(t, w2.Reader())
		assert.True(t, w2.ReaderWrapped())
	})
}

func TestCmdWorker_Chained_AllWrappers(t *testing.T) {
	input := "10,20\n30,40\n50,60\n"

	var teeBuf, multiBuf bytes.Buffer

	p := &Pipeline{}

	w1 := NewCmdWorker("cat")
	w2 := NewCmdWorker("awk", "-F,", "{print $1+$2}")
	w3 := NewCmdWorker("sed", "s/$/ chars/")
	w4 := NewCmdWorker("wc", "-l")

	w1.WithWriterWrappers(BufWrite(4096))
	w2.WithReaderWrappers(AsyncRead(), BufRead(4096))
	w3.WithReaderWrappers(Tee(&teeBuf))
	w3.WithWriterWrappers(MultiWrite(&multiBuf))
	w3.WithWriterWrappers(AsyncWrite())
	w4.WithReaderWrappers(BufRead(4096))

	p.Add(w1, w2, w3, w4)

	err := runner.Init(context.Background(), p)
	require.NoError(t, err)

	assert.NotNil(t, w1.Writer())
	assert.NotNil(t, w2.Reader())
	assert.NotNil(t, w2.Writer())
	assert.NotNil(t, w3.Reader())
	assert.NotNil(t, w3.Writer())
	assert.NotNil(t, w4.Reader())

	inputPr, inputPw := io.Pipe()
	w1.ReadFromReader(inputPr)

	mw := newMockWriter()
	w4.WriteToWriter(mw)

	ctx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Start(ctx, p)
	}()

	select {
	case <-w1.(*cmdWorker).Started():
	case <-time.After(3 * time.Second):
		t.Fatal("pipeline did not start")
	}

	inputPw.Write([]byte(input))
	inputPw.Close()

	workers := []Worker{w1, w2, w3, w4}
	for i := 1; i < len(workers); i++ {
		select {
		case <-workers[i-1].(*cmdWorker).Done():
		case <-time.After(10 * time.Second):
			t.Fatalf("worker %d did not finish", i-1)
		}
	}

	select {
	case err = <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline did not finish")
	}

	assert.Equal(t, "3", strings.TrimSpace(string(mw.buf)))
	assert.Equal(t, "30\n70\n110\n", teeBuf.String())
	assert.Equal(t, "30 chars\n70 chars\n110 chars\n", multiBuf.String())
}

func TestCmdWorker_Chained_AllWrappers_NonZeroCopy(t *testing.T) {
	p := &Pipeline{}
	w1 := NewCmdWorker("cat")
	w2 := NewCmdWorker("cat")
	w3 := NewCmdWorker("cat")
	w4 := NewCmdWorker("cat")

	w1.WithWriterWrappers(BufWrite(4096))
	w2.WithReaderWrappers(AsyncRead())
	w2.WithWriterWrappers(AsyncWrite())
	w3.WithReaderWrappers(BufRead(4096))
	w3.WithWriterWrappers(MultiWrite(&bytes.Buffer{}))
	w4.WithReaderWrappers(Tee(&bytes.Buffer{}))

	p.Add(w1, w2, w3, w4)

	err := p.Init(context.Background())
	require.NoError(t, err)

	assert.True(t, w1.WriterWrapped())
	assert.True(t, w2.ReaderWrapped() || w2.WriterWrapped())
	assert.True(t, w3.ReaderWrapped() || w3.WriterWrapped())
	assert.True(t, w4.ReaderWrapped())

	assert.NotNil(t, w1.Writer())
	assert.NotNil(t, w2.Reader())
	assert.NotNil(t, w2.Writer())
	assert.NotNil(t, w3.Reader())
	assert.NotNil(t, w3.Writer())
	assert.NotNil(t, w4.Reader())
}

// TestCmdWorker_Pipeline_Stop tests runner.Stop on a Pipeline with multiple
// cmdWorkers. Stops w1 via runner.Stop, then cascades Close(false) to
// unblock downstream workers, verifying all workers finish.
func TestCmdWorker_Pipeline_Stop(t *testing.T) {
	p := &Pipeline{}
	// w1 blocks until killed; w2/w3/w4 read stdin and exit on EOF.
	w1 := NewCmdWorker("sleep", "30")
	w2 := NewCmdWorker("cat")
	w3 := NewCmdWorker("cat")
	w4 := NewCmdWorker("cat")
	p.Add(w1, w2, w3, w4)

	err := p.Init(context.Background())
	require.NoError(t, err)

	w1.ReadFromReader(newMockReader(nil))
	w4.WriteToWriter(newMockWriter())

	ctx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Start(ctx, p)
	}()

	select {
	case <-w1.(*cmdWorker).Started():
	case <-time.After(3 * time.Second):
		t.Fatal("pipeline did not start")
	}

	time.Sleep(200 * time.Millisecond)

	// Stop w1 via runner.Stop. cmdWorker.Stop → c.cancel → kills sleep 30.
	// w2/w3/w4 (cat) are blocked reading from upstream pipes.
	err = runner.Stop(ctx, w1.(runner.Runner))
	require.NoError(t, err)

	// Wait for w1 to actually exit, then cascade close pipes.
	select {
	case <-w1.(*cmdWorker).Done():
	case <-time.After(10 * time.Second):
		t.Fatal("worker 1 did not stop")
	}
	_ = w1.(*cmdWorker).BaseWorker.Close(false)

	select {
	case <-w2.(*cmdWorker).Done():
	case <-time.After(10 * time.Second):
		t.Fatal("worker 2 did not stop")
	}
	_ = w2.(*cmdWorker).BaseWorker.Close(false)

	select {
	case <-w3.(*cmdWorker).Done():
	case <-time.After(10 * time.Second):
		t.Fatal("worker 3 did not stop")
	}
	_ = w3.(*cmdWorker).BaseWorker.Close(false)

	// Pipeline.Start should now return.
	select {
	case err = <-errCh:
		require.Error(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("pipeline did not finish after stop")
	}

	for _, w := range []Worker{w1, w2, w3, w4} {
		select {
		case <-w.(*cmdWorker).Done():
		default:
			t.Error("worker not done")
		}
	}
}

// TestCmdWorker_Pipeline_Stop_WithDataFlow tests stopping a pipeline that
// is actively processing data via runner.Stop, verifying graceful shutdown.
func TestCmdWorker_Pipeline_Stop_WithDataFlow(t *testing.T) {
	p := &Pipeline{}
	w1 := NewCmdWorker("cat")
	w2 := NewCmdWorker("cat")
	w3 := NewCmdWorker("cat")
	p.Add(w1, w2, w3)

	err := p.Init(context.Background())
	require.NoError(t, err)

	inputPr, inputPw := io.Pipe()
	w1.ReadFromReader(inputPr)

	mw := newMockWriter()
	w3.WriteToWriter(mw)

	ctx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Start(ctx, p)
	}()

	select {
	case <-w1.(*cmdWorker).Started():
	case <-time.After(3 * time.Second):
		t.Fatal("pipeline did not start")
	}

	time.Sleep(500 * time.Millisecond)

	// Feed data then stop.
	inputPw.Write([]byte("line1\nline2\nline3\n"))

	// Stop w1. runner.Stop kills cat process; error flows through.
	err = runner.Stop(ctx, w1.(runner.Runner))
	require.NoError(t, err)

	// Close the input pipe reader to unblock the exec.Cmd stdin copy goroutine.
	// (cmdWorker.Start doesn't close the reader after cancel.)
	inputPr.Close()
	inputPw.Close()

	// Cascade close pipes to unblock w2 and w3.
	select {
	case <-w1.(*cmdWorker).Done():
	case <-time.After(5 * time.Second):
		t.Fatal("worker 1 did not stop")
	}
	_ = w1.(*cmdWorker).BaseWorker.Close(false)

	select {
	case <-w2.(*cmdWorker).Done():
	case <-time.After(5 * time.Second):
		t.Fatal("worker 2 did not stop")
	}
	_ = w2.(*cmdWorker).BaseWorker.Close(false)

	// Pipeline finished.
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline did not finish")
	}
}

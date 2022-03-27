package v2

import (
	"bufio"
	"errors"
	"io"
	"time"

	"github.com/donkeywon/golib/aio"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

var CreateWriter = newBaseWriter

type WriterWrapper interface {
	Wrap(io.WriteCloser)
}

type WriterType string

type Writer interface {
	io.WriteCloser
	io.ReaderFrom
	runner.Runner
	plugin.Plugin

	Wrap(io.WriteCloser)
	MultiWrite(...io.Writer)
	EnableBuf(bufSize int)
	EnableAsync(bufSize int, queueSize int, deadline time.Duration)
}

type nopWriteCloser struct {
	io.Writer
}

type flusher interface {
	Flush() error
}

type flusher2 interface {
	Flush()
}

func (n nopWriteCloser) Close() error {
	switch t := n.Writer.(type) {
	case flusher:
		return t.Flush()
	case flusher2:
		t.Flush()
	}
	return nil
}

type BaseWriter struct {
	runner.Runner
	io.WriteCloser

	originWriter io.WriteCloser

	ws          []io.Writer
	bufSize     int
	queueSize   int
	enableBuf   bool
	enableAsync bool
	deadline    time.Duration
}

func newBaseWriter(name string) Writer {
	return &BaseWriter{
		Runner: runner.Create(name),
	}
}

func (b *BaseWriter) Init() error {
	b.originWriter = b.WriteCloser
	if len(b.ws) > 0 {
		b.originWriter = b.WriteCloser
		ws := make([]io.Writer, 0, 1+len(b.ws))
		ws = append(ws, b.WriteCloser)
		ws = append(ws, b.ws...)
		b.WriteCloser = nopWriteCloser{io.MultiWriter(ws...)}
	}

	if b.enableBuf {
		b.WriteCloser = nopWriteCloser{bufio.NewWriterSize(b.WriteCloser, b.bufSize)}
	} else if b.enableAsync {
		b.WriteCloser = nopWriteCloser{aio.NewAsyncWriter(b.WriteCloser, aio.BufSize(b.bufSize), aio.QueueSize(b.queueSize), aio.Deadline(b.deadline))}
	}

	return b.Runner.Init()
}

func (b *BaseWriter) Close() error {
	defer b.Cancel()
	if b.originWriter != nil && b.originWriter != b.WriteCloser {
		return errors.Join(b.WriteCloser.Close(), b.originWriter.Close())
	}
	if b.WriteCloser != nil {
		return b.WriteCloser.Close()
	}
	return nil
}

func (b *BaseWriter) Wrap(w io.WriteCloser) {
	if w == nil {
		panic(ErrWrapNil)
	}
	if b.WriteCloser != nil {
		panic(ErrWrapTwice)
	}
	b.WriteCloser = w
}

func (b *BaseWriter) MultiWrite(w ...io.Writer) {
	b.ws = append(b.ws, w...)
}

func (b *BaseWriter) EnableBuf(bufSize int) {
	b.enableBuf = true
	b.bufSize = bufSize
}

func (b *BaseWriter) EnableAsync(bufSize int, queueSize int, deadline time.Duration) {
	b.enableAsync = true
	b.bufSize = bufSize
	b.queueSize = queueSize
	b.deadline = deadline
}

func (b *BaseWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := b.WriteCloser.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}

	return io.Copy(b.WriteCloser, r)
}

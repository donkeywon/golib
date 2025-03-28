package pipeline

import (
	"bufio"
	"errors"
	"io"

	"github.com/donkeywon/golib/aio"
	"github.com/donkeywon/golib/runner"
)

var CreateWriter = newBaseWriter

type WriterWrapper interface {
	Wrap(io.WriteCloser)
}

type Writer interface {
	Common
	io.Writer
	io.ReaderFrom

	WrapWriter(io.WriteCloser)
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

	opt *option
}

func newBaseWriter(name string) Writer {
	return &BaseWriter{
		Runner: runner.Create(name),
		opt:    newOption(),
	}
}

func (b *BaseWriter) Init() error {
	b.originWriter = b.WriteCloser
	if len(b.opt.ws) > 0 {
		b.originWriter = b.WriteCloser
		ws := make([]io.Writer, 0, 1+len(b.opt.ws))
		ws = append(ws, b.WriteCloser)
		ws = append(ws, b.opt.ws...)
		b.WriteCloser = nopWriteCloser{io.MultiWriter(ws...)}
	}

	if b.opt.enableBuf {
		b.WriteCloser = nopWriteCloser{bufio.NewWriterSize(b.WriteCloser, b.opt.bufSize)}
	} else if b.opt.enableAsync {
		b.WriteCloser = nopWriteCloser{aio.NewAsyncWriter(b.WriteCloser, aio.BufSize(b.opt.bufSize), aio.QueueSize(b.opt.queueSize), aio.Deadline(b.opt.deadline))}
	}

	return b.Runner.Init()
}

func (b *BaseWriter) Close() error {
	defer b.Cancel()
	if b.originWriter != nil && b.originWriter != b.WriteCloser {
		return errors.Join(b.WriteCloser.Close(), b.originWriter.Close(), b.opt.onclose())
	}
	if b.WriteCloser != nil {
		return errors.Join(b.WriteCloser.Close(), b.opt.onclose())
	}
	return nil
}

func (b *BaseWriter) WrapWriter(w io.WriteCloser) {
	if w == nil {
		panic(ErrWrapNil)
	}
	if b.WriteCloser != nil {
		panic(ErrWrapTwice)
	}
	b.WriteCloser = w
}

func (b *BaseWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := b.WriteCloser.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}

	return io.Copy(b.WriteCloser, r)
}

func (b *BaseWriter) WithOptions(opts ...Option) {
	b.opt.with(opts...)
}

func (b *BaseWriter) Type() Type {
	panic("not implemented")
}

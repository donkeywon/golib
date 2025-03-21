package v2

import (
	"io"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

type WriterType string

type Writer interface {
	io.WriteCloser
	runner.Runner
	plugin.Plugin

	MultiWrite(w ...io.Writer)
	Wrap(func(io.WriteCloser) io.WriteCloser)
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

type BaseWriter struct {
	runner.Runner
	io.WriteCloser

	originWriter io.WriteCloser
	mw           []io.Writer
}

func NewBaseWriter(name string) Writer {
	return &BaseWriter{
		Runner: runner.Create(name),
	}
}

func (b *BaseWriter) Init() error {
	if len(b.mw) > 0 {
		b.originWriter = b.WriteCloser
		ws := make([]io.Writer, 0, 1+len(b.mw))
		ws = append(ws, b.WriteCloser)
		ws = append(ws, b.mw...)
		b.WriteCloser = nopWriteCloser{io.MultiWriter(ws...)}
	}

	return b.Runner.Init()
}

func (b *BaseWriter) MultiWrite(w ...io.Writer) {
	b.mw = append(b.mw, w...)
}

func (b *BaseWriter) Wrap(f func(io.WriteCloser) io.WriteCloser) {
	b.WriteCloser = f(b.WriteCloser)
}

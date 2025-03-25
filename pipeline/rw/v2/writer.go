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

	Wrap(io.WriteCloser)
	WithOptions(opts ...Option)
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

type BaseWriter struct {
	runner.Runner
	io.WriteCloser

	originWriter io.WriteCloser
	opt          *option
}

func NewBaseWriter(name string, opts ...Option) Writer {
	w := &BaseWriter{
		Runner: runner.Create(name),
		opt:    newOption(),
	}
	for _, opt := range opts {
		opt(w.opt)
	}
	return w
}

func (b *BaseWriter) Init() error {
	if len(b.opt.ws) > 0 {
		b.originWriter = b.WriteCloser
		ws := make([]io.Writer, 0, 1+len(b.opt.ws))
		ws = append(ws, b.WriteCloser)
		ws = append(ws, b.opt.ws...)
		b.WriteCloser = nopWriteCloser{io.MultiWriter(ws...)}
	}

	return b.Runner.Init()
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

func (b *BaseWriter) WithOptions(opts ...Option) {
	b.opt.with(opts...)
}

package pipeline

import (
	"errors"
	"io"
	"slices"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

var CreateWriter = newBaseWriter

type writerWrapper interface {
	WrapWriter(io.Writer)
}

type WriterCfg struct {
	CommonCfg
	CommonOption
}

func (wc *WriterCfg) build() Writer {
	w := plugin.CreateWithCfg[Type, Writer](wc.Type, wc.Cfg)
	w.WithOptions(wc.toOptions(true)...)
	return w
}

type Writer interface {
	Common
	io.Writer
	io.ReaderFrom
	writerWrapper
}

type flusher interface {
	Flush() error
}

type flusher2 interface {
	Flush()
}

type flushOnClose struct {
	f  flusher
	f2 flusher2
}

func (f *flushOnClose) Close() error {
	if f.f != nil {
		return f.f.Flush()
	} else if f.f2 != nil {
		f.f2.Flush()
	}
	return nil
}

type nopWriteCloser struct {
	io.Writer
}

func (n *nopWriteCloser) Close() error {
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
	io.Writer

	originWriter io.Writer
	closes       []closeFunc

	opt *option
}

func newBaseWriter(name string) Writer {
	return &BaseWriter{
		Runner: runner.Create(name),
		opt:    newOption(),
	}
}

func (b *BaseWriter) Init() error {
	b.originWriter = b.Writer
	b.appendCloses(b.originWriter)
	if len(b.opt.ws) > 0 {
		ws := make([]io.Writer, 0, 1+len(b.opt.ws))
		ws = append(ws, b.Writer)
		ws = append(ws, b.opt.ws...)
		b.Writer = io.MultiWriter(ws...)
	}

	if len(b.opt.writerWrapFuncs) > 0 {
		for _, f := range b.opt.writerWrapFuncs {
			b.Writer = f(b.Writer)
			b.appendCloses(b.Writer)
		}
	}

	slices.Reverse(b.closes)

	return b.Runner.Init()
}

func (b *BaseWriter) appendCloses(w io.Writer) {
	if c, ok := w.(io.Closer); ok {
		b.closes = append(b.closes, c.Close)
	}
	switch f := w.(type) {
	case flusher:
		b.closes = append(b.closes, f.Flush)
	case flusher2:
		b.closes = append(b.closes, func() error {
			f.Flush()
			return nil
		})
	}
}

func (b *BaseWriter) Close() error {
	defer b.Cancel()

	return errors.Join(doAllClose(b.closes), b.opt.onclose())
}

func (b *BaseWriter) WrapWriter(w io.Writer) {
	if w == nil {
		panic(ErrWrapNil)
	}
	if b.Writer != nil {
		panic(ErrWrapTwice)
	}
	b.Writer = w
}

func (b *BaseWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := b.Writer.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}

	return io.Copy(b.Writer, r)
}

func (b *BaseWriter) WithOptions(opts ...Option) {
	b.opt.with(opts...)
}

func (b *BaseWriter) Type() Type {
	panic("not implemented")
}

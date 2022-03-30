package pipeline

import (
	"errors"
	"io"
	"slices"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

var CreateReader = newBaseReader

type canceler interface {
	Cancel()
}

type optionApplier interface {
	WithOptions(...Option)
}

type readerWrapper interface {
	WrapReader(io.Reader)
}

type Reader interface {
	Common
	io.Reader
	io.WriterTo
	readerWrapper
}

type ReaderCfg struct {
	CommonCfg
	CommonOption
}

func (rc *ReaderCfg) build() Reader {
	r := plugin.CreateWithCfg[Type, Reader](rc.Type, rc.Cfg)
	r.WithOptions(rc.toOptions(false)...)
	return r
}

type BaseReader struct {
	runner.Runner
	io.Reader

	originReader io.Reader
	closes       []closeFunc

	opt *option
}

func newBaseReader(name string) Reader {
	return &BaseReader{
		Runner: runner.Create(name),
		opt:    newOption(),
	}
}

func (b *BaseReader) Init() error {
	b.originReader = b.Reader
	b.appendCloses(b.originReader)
	if len(b.opt.tees) > 0 {
		b.Reader = io.NopCloser(io.TeeReader(b.Reader, io.MultiWriter(b.opt.tees...)))
	}

	if len(b.opt.readerWrapFuncs) > 0 {
		for _, f := range b.opt.readerWrapFuncs {
			b.Reader = f(b.Reader)
			b.appendCloses(b.Reader)
		}
	}

	slices.Reverse(b.closes)

	return b.Runner.Init()
}

func (b *BaseReader) appendCloses(r io.Reader) {
	if c, ok := r.(io.Closer); ok {
		b.closes = append(b.closes, c.Close)
	}
}

func (b *BaseReader) Close() error {
	defer b.Cancel()
	return errors.Join(doAllClose(b.closes), b.opt.onclose())
}

func (b *BaseReader) WrapReader(r io.Reader) {
	if r == nil {
		panic(ErrWrapNil)
	}
	if b.Reader != nil {
		panic(ErrWrapTwice)
	}
	b.Reader = r
}

func (b *BaseReader) Type() Type { panic("not implemented") }

func (b *BaseReader) WriteTo(w io.Writer) (int64, error) {
	if wt, ok := b.Reader.(io.WriterTo); ok {
		return wt.WriteTo(w)
	}

	return io.Copy(w, b.Reader)
}

func (b *BaseReader) WithOptions(opts ...Option) {
	b.opt.with(opts...)
}

func (b *BaseReader) Size() int64 {
	switch t := b.originReader.(type) {
	case hasSize:
		return t.Size()
	case hasSize2:
		return int64(t.Size())
	default:
		return 0
	}
}

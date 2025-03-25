package v2

import (
	"io"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

type ReaderType string

type Reader interface {
	io.ReadCloser
	runner.Runner
	plugin.Plugin

	Wrap(io.ReadCloser)
	WithOptions(...Option)
}

type BaseReader struct {
	runner.Runner
	io.ReadCloser

	originReader io.ReadCloser
	opt          *option
}

func NewBaseReader(name string, opts ...Option) Reader {
	r := &BaseReader{
		Runner: runner.Create(name),
		opt:    newOption(),
	}
	for _, opt := range opts {
		opt(r.opt)
	}
	return r
}

func (b *BaseReader) Init() error {
	if len(b.opt.tees) > 0 {
		b.originReader = b.ReadCloser
		b.ReadCloser = io.NopCloser(io.TeeReader(b.ReadCloser, io.MultiWriter(b.opt.tees...)))
	}

	return b.Runner.Init()
}

func (b *BaseReader) Wrap(r io.ReadCloser) {
	if r == nil {
		panic(ErrWrapNil)
	}
	if b.ReadCloser != nil {
		panic(ErrWrapTwice)
	}
	b.ReadCloser = r
}

func (b *BaseReader) WithOptions(opts ...Option) {
	b.opt.with(opts...)
}

func (b *BaseReader) Type() any { panic("not implemented") }

func (b *BaseReader) GetCfg() any { panic("not implemented") }

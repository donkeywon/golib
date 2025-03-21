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

	Tee(w ...io.Writer)
	Wrap(func(io.ReadCloser) io.ReadCloser)
}

type BaseReader struct {
	runner.Runner
	io.ReadCloser

	originReader io.ReadCloser
	tees         []io.Writer
}

func NewBaseReader(name string) Reader {
	return &BaseReader{
		Runner: runner.Create(name),
	}
}

func (b *BaseReader) Init() error {
	if len(b.tees) > 0 {
		b.originReader = b.ReadCloser
		b.ReadCloser = io.NopCloser(io.TeeReader(b.ReadCloser, io.MultiWriter(b.tees...)))
	}

	return b.Runner.Init()
}

func (b *BaseReader) Tee(w ...io.Writer) {
	b.tees = append(b.tees, w...)
}

func (b *BaseReader) Wrap(f func(io.ReadCloser) io.ReadCloser) {
	b.ReadCloser = f(b.ReadCloser)
}

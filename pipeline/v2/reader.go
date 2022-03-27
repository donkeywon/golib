package v2

import (
	"bufio"
	"errors"
	"io"

	"github.com/donkeywon/golib/aio"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

type canceler interface {
	Cancel()
}

var CreateReader = newBaseReader

type ReaderWrapper interface {
	Wrap(io.ReadCloser)
}

type ReaderType string

type Reader interface {
	io.ReadCloser
	io.WriterTo
	runner.Runner
	plugin.Plugin

	Wrap(io.ReadCloser)
	WithOptions(...Option)
}

type BaseReader struct {
	runner.Runner
	io.ReadCloser

	originReader io.ReadCloser

	opt *option
}

func newBaseReader(name string) Reader {
	return &BaseReader{
		Runner: runner.Create(name),
		opt:    newOption(),
	}
}

func (b *BaseReader) Init() error {
	b.originReader = b.ReadCloser
	if len(b.opt.tees) > 0 {
		b.ReadCloser = io.NopCloser(io.TeeReader(b.ReadCloser, io.MultiWriter(b.opt.tees...)))
	}

	if b.opt.enableAsync {
		b.ReadCloser = aio.NewAsyncReader(b.ReadCloser, aio.BufSize(b.opt.bufSize), aio.QueueSize(b.opt.queueSize))
	} else if b.opt.enableBuf {
		b.ReadCloser = io.NopCloser(bufio.NewReaderSize(b.ReadCloser, b.opt.bufSize))
	}

	return b.Runner.Init()
}

func (b *BaseReader) Close() error {
	defer b.Cancel()

	if b.originReader != nil && b.originReader != b.ReadCloser {
		return errors.Join(b.ReadCloser.Close(), b.originReader.Close())
	}
	if b.ReadCloser != nil {
		return b.ReadCloser.Close()
	}
	return nil
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

func (b *BaseReader) Type() any { panic("not implemented") }

func (b *BaseReader) GetCfg() any { panic("not implemented") }

func (b *BaseReader) WriteTo(w io.Writer) (int64, error) {
	if wt, ok := b.ReadCloser.(io.WriterTo); ok {
		return wt.WriteTo(w)
	}

	return io.Copy(w, b.ReadCloser)
}

func (b *BaseReader) WithOptions(opts ...Option) {
	for _, opt := range opts {
		opt(b.opt)
	}
}

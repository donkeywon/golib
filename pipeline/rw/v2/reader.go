package v2

import (
	"bufio"
	"errors"
	"io"

	"github.com/donkeywon/golib/aio"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

var CreateReader = newBaseReader

type ReaderWrapper interface {
	Wrap(io.ReadCloser)
}

type ReaderType string

type Reader interface {
	io.ReadCloser
	runner.Runner
	plugin.Plugin

	Wrap(io.ReadCloser)
}

type BaseReader struct {
	runner.Runner
	io.ReadCloser

	originReader io.ReadCloser

	tees        []io.Writer
	bufSize     int
	queueSize   int
	enableBuf   bool
	enableAsync bool
}

func newBaseReader(name string) Reader {
	return &BaseReader{
		Runner: runner.Create(name),
	}
}

func (b *BaseReader) Init() error {
	b.originReader = b.ReadCloser
	if len(b.tees) > 0 {
		b.ReadCloser = io.NopCloser(io.TeeReader(b.ReadCloser, io.MultiWriter(b.tees...)))
	}

	if b.enableAsync {
		b.ReadCloser = aio.NewAsyncReader(b.ReadCloser, aio.BufSize(b.bufSize), aio.QueueSize(b.queueSize))
	} else if b.enableBuf {
		b.ReadCloser = io.NopCloser(bufio.NewReaderSize(b.ReadCloser, b.bufSize))
	}

	return b.Runner.Init()
}

func (b *BaseReader) Close() error {
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

func (b *BaseReader) Tee(w ...io.Writer) {
	b.tees = append(b.tees, w...)
}

func (b *BaseReader) EnableBuf(bufSize int) {
	b.enableBuf = true
	b.bufSize = bufSize
}

func (b *BaseReader) EnableAsync(bufSize int, queueSize int) {
	b.enableAsync = true
	b.bufSize = bufSize
	b.queueSize = queueSize
}

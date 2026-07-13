package pipeline

import (
	"bufio"
	"io"

	"github.com/donkeywon/golib/aio"
)

type ReaderWrapFunc func(io.Reader) io.Reader

type WriterWrapFunc func(io.Writer) io.Writer

func BufRead(size int) ReaderWrapFunc {
	return func(r io.Reader) io.Reader {
		return bufio.NewReaderSize(r, size)
	}
}

func BufWrite(size int) WriterWrapFunc {
	return func(w io.Writer) io.Writer {
		return bufio.NewWriterSize(w, size)
	}
}

func AsyncWrite(opts ...aio.Option) WriterWrapFunc {
	return func(w io.Writer) io.Writer {
		return aio.NewAsyncWriter(w, opts...)
	}
}

func AsyncRead(opts ...aio.Option) ReaderWrapFunc {
	return func(r io.Reader) io.Reader {
		return aio.NewAsyncReader(r, opts...)
	}
}

func Tee(w ...io.Writer) ReaderWrapFunc {
	return func(r io.Reader) io.Reader {
		return io.TeeReader(r, io.MultiWriter(w...))
	}
}

func MultiWrite(ws ...io.Writer) WriterWrapFunc {
	return func(w io.Writer) io.Writer {
		wss := make([]io.Writer, 0, len(ws)+1)
		wss = append(wss, w)
		wss = append(wss, ws...)
		return io.MultiWriter(wss...)
	}
}

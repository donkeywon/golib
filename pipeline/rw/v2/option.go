package v2

import "io"

type Option func(*option)

type ReaderWrapper func(r io.ReadCloser) io.ReadCloser
type WriterWrapper func(w io.WriteCloser) io.WriteCloser

type option struct {
	enableBuf bool
	bufSize   int

	enableAsync bool
	queueSize   int

	tees []io.Writer
	ws   []io.Writer

	readerWrappers []ReaderWrapper
	writerWrappers []WriterWrapper
}

func newOption() *option {
	return &option{}
}

func (o *option) with(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

func EnableBuf(bufSize int) Option {
	return func(o *option) {
		o.enableBuf = true
		o.bufSize = bufSize
	}
}

func EnableAsync(bufSize int, queueSize int) Option {
	return func(o *option) {
		o.enableAsync = true
		o.bufSize = bufSize
		o.queueSize = queueSize
	}
}

// Tee only for Reader
func Tee(w ...io.Writer) Option {
	return func(o *option) {
		o.tees = append(o.tees, w...)
	}
}

// MultiWrite only for Writer
func MultiWrite(w ...io.Writer) Option {
	return func(o *option) {
		o.ws = append(o.ws, w...)
	}
}

func WrapReader(f ...ReaderWrapper) Option {
	return func(o *option) {
		o.readerWrappers = append(o.readerWrappers, f...)
	}
}

func WrapWriter(f ...WriterWrapper) Option {
	return func(o *option) {
		o.writerWrappers = append(o.writerWrappers, f...)
	}
}

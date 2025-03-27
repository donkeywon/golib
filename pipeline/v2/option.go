package v2

import (
	"io"
	"time"
)

type Option func(*option)

type option struct {
	enableBuf bool
	bufSize   int

	enableAsync bool
	queueSize   int
	deadline    time.Duration

	tees []io.Writer
	ws   []io.Writer
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

// EnableAsyncDeadline Only for Writer
func EnableAsyncDeadline(bufSize int, queueSize int, deadline time.Duration) Option {
	return func(o *option) {
		o.enableAsync = true
		o.bufSize = bufSize
		o.queueSize = queueSize
		o.deadline = deadline
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

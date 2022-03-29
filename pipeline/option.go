package pipeline

import (
	"hash"
	"io"
	"time"
)

type itemSetter interface {
	Set(Common)
}

type Option interface {
	apply(*option)
}

type optionFunc func(*option)

func (f optionFunc) apply(o *option) {
	f(o)
}

type option struct {
	enableBuf bool
	bufSize   int

	enableAsync bool
	queueSize   int
	deadline    time.Duration

	tees []io.Writer
	ws   []io.Writer

	onclose []func() error
}

func newOption() *option {
	return &option{}
}

func (o *option) with(opts ...Option) {
	for _, opt := range opts {
		opt.apply(o)
	}
}

func EnableBuf(bufSize int) Option {
	return optionFunc(func(o *option) {
		o.enableBuf = true
		o.bufSize = bufSize
	})
}

func EnableAsync(bufSize int, queueSize int) Option {
	return optionFunc(func(o *option) {
		o.enableAsync = true
		o.bufSize = bufSize
		o.queueSize = queueSize
	})
}

// EnableAsyncDeadline Only for Writer
func EnableAsyncDeadline(bufSize int, queueSize int, deadline time.Duration) Option {
	return optionFunc(func(o *option) {
		o.enableAsync = true
		o.bufSize = bufSize
		o.queueSize = queueSize
		o.deadline = deadline
	})
}

// Tee only for Reader
func Tee(w ...io.Writer) Option {
	return optionFunc(func(o *option) {
		o.tees = append(o.tees, w...)
	})
}

// MultiWrite only for Writer
func MultiWrite(w ...io.Writer) Option {
	return optionFunc(func(o *option) {
		o.ws = append(o.ws, w...)
	})
}

func OnClose(f ...func() error) Option {
	return optionFunc(func(o *option) {
		o.onclose = append(o.onclose, f...)
	})
}

type logWrite struct {
	Common
}

func (l *logWrite) Write(p []byte) (int, error) {
	l.Info("write", "len", len(p))
	return len(p), nil
}

func (l *logWrite) Set(c Common) {
	l.Common = c
}

func LogWrite(lvl string) Option {
	return MultiWrite(&logWrite{})
}

func Hash(h hash.Hash) Option {

}

func Checksum(checksum string, h hash.Hash) Option {

}

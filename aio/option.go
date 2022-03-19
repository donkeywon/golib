package aio

import "time"

type Option func(*option)

func (o Option) apply(r *option) {
	o(r)
}

type option struct {
	bufSize   int
	queueSize int
	deadline  time.Duration
}

func newOption() *option {
	return &option{
		bufSize: 1024 * 1024,
	}
}

func BufSize(bufSize int) Option {
	return func(o *option) {
		if bufSize > 0 {
			o.bufSize = bufSize
		}
	}
}

func QueueSize(queueSize int) Option {
	return func(o *option) {
		if queueSize > 0 {
			o.queueSize = queueSize
		}
	}
}

// Deadline Only for AsyncWriter Flush.
func Deadline(deadline time.Duration) Option {
	return func(o *option) {
		if deadline > 0 {
			o.deadline = deadline
		}
	}
}

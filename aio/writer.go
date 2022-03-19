package aio

import (
	"io"
	"sync"
	"time"
)

// TODO lock

type AsyncWriter struct {
	w   io.Writer
	opt *option

	buf   *buf
	queue chan *buf

	asyncWriteOnce sync.Once
	closeOnce      sync.Once
	closed         chan struct{}
	asyncDone      chan struct{}

	deadlineTimer *time.Timer

	err error
}

func NewAsyncWriter(w io.Writer, opts ...Option) *AsyncWriter {
	aw := &AsyncWriter{
		w:         w,
		opt:       newOption(),
		closed:    make(chan struct{}),
		asyncDone: make(chan struct{}),
	}
	for _, o := range opts {
		o.apply(aw.opt)
	}
	aw.queue = make(chan *buf, aw.opt.queueSize)
	return aw
}

func (aw *AsyncWriter) Write(p []byte) (n int, err error) {
	select {
	case <-aw.closed:
		return 0, aw.err
	default:
	}

	if aw.err != nil {
		return 0, aw.err
	}

	aw.asyncWriteOnce.Do(func() {
		go aw.asyncWrite()
		if aw.opt.deadline > 0 {
			aw.deadlineTimer = time.NewTimer(aw.opt.deadline)
			go aw.deadline()
		}
	})

	var nn int
	for len(p) > 0 {
		aw.prepareBuf()
		nn = aw.buf.readFrom(p)
		p = p[nn:]

		if aw.buf.isWriteFull() {
			err = aw.Flush()
			if err != nil {
				break
			}
		}
	}

	return
}

func (aw *AsyncWriter) Close() error {
	// TODO
	return nil
}

func (aw *AsyncWriter) ReadFrom(r io.Reader) (n int64, err error) {
	// TODO
	return
}

func (aw *AsyncWriter) Flush() error {
	if aw.err != nil {
		return aw.err
	}

	if aw.buf == nil || aw.buf.isNotWritten() {
		return nil
	}

	aw.queue <- aw.buf
	aw.buf = nil
	if aw.deadlineTimer != nil {
		aw.deadlineTimer.Reset(aw.opt.deadline)
	}
	return nil
}

func (aw *AsyncWriter) prepareBuf() {
	if aw.buf != nil && !aw.buf.isWriteFull() {
		return
	}

	aw.buf = newBuf(aw.opt.bufSize)
}

func (aw *AsyncWriter) asyncWrite() {
	var err error
	for {
		b, ok := <-aw.queue
		if !ok {
			return
		}

		if aw.err != nil {
			b.free()
			continue
		}

		for !b.isReadCompletely() && err == nil {
			_, err = b.writeTo(aw.w)
		}
	}
}

func (aw *AsyncWriter) deadline() {
	for {
		select {
		case <-aw.closed:
			return
		case <-aw.deadlineTimer.C:
			if aw.err != nil {
				return
			}

			err := aw.Flush()
			if err != nil {
				aw.err = err
				return
			}
		}
	}
}

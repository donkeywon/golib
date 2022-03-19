package aio

import (
	"io"
	"sync"
)

type AsyncWriter struct {
	w   io.Writer
	opt *option

	buf   *buf
	queue chan *buf

	asyncWriteOnce sync.Once
	closeOnce      sync.Once
	closed         chan struct{}

	err error
}

func NewAsyncWriter(w io.Writer, opts ...Option) *AsyncWriter {
	aw := &AsyncWriter{
		w:   w,
		opt: newOption(),
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
	})

	var nn int
	for len(p) > 0 {
		aw.prepareBuf()
		nn = aw.buf.readFrom(p)
		p = p[nn:]

		if aw.buf.isFull() {
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

	// TODO flush
	return nil
}

func (aw *AsyncWriter) prepareBuf() {
	if aw.buf != nil && !aw.buf.isFull() {
		return
	}

	aw.buf = newBuf(aw.opt.bufSize)
}

func (aw *AsyncWriter) asyncWrite() {

}

func (aw *AsyncWriter) deadline() {

}

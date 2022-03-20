package aio

import (
	"io"
	"sync"
	"time"

	"github.com/donkeywon/golib/util/buffer"
)

type AsyncWriter struct {
	w   io.Writer
	opt *option

	mu    sync.Mutex
	buf   *buffer.FixedBuffer
	queue chan *buffer.FixedBuffer

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
	aw.queue = make(chan *buffer.FixedBuffer, aw.opt.queueSize)
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

	aw.initOnce()

	var nn int
	for len(p) > 0 {
		aw.prepareBuf()
		nn, err = aw.bufWrite(p)
		p = p[nn:]

		// buf full
		if err == io.ErrShortWrite {
			err = aw.Flush()
			if err != nil {
				break
			}
		}
	}

	return
}

func (aw *AsyncWriter) Close() error {
	var err error
	aw.closeOnce.Do(func() {
		close(aw.closed)

		err = aw.Flush()
		close(aw.queue)
		<-aw.asyncDone

		if err == nil {
			err = aw.err
		}
	})
	return err
}

func (aw *AsyncWriter) ReadFrom(r io.Reader) (n int64, err error) {
	select {
	case <-aw.closed:
		return 0, aw.err
	default:
	}

	if aw.err != nil {
		return 0, aw.err
	}

	aw.initOnce()

	var nn int64
	for {
		aw.prepareBuf()
		nn, err = aw.bufReadFrom(r)
		n += nn
		// buf full
		if err == io.ErrShortWrite {
			err = aw.Flush()
			if err != nil {
				return
			}
		}

		if err != nil {
			return
		}
	}
}

func (aw *AsyncWriter) Flush() error {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	if aw.err != nil {
		return aw.err
	}

	if aw.buf == nil || aw.buf.Len() == 0 {
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
	aw.mu.Lock()
	defer aw.mu.Unlock()

	if aw.buf != nil && aw.buf.Len() < aw.buf.Cap() {
		// not full
		return
	}

	aw.buf = buffer.NewFixedBuffer(aw.opt.bufSize)
}

func (aw *AsyncWriter) bufWrite(p []byte) (int, error) {
	aw.mu.Lock()
	defer aw.mu.Unlock()
	return aw.buf.Write(p)
}

func (aw *AsyncWriter) bufReadFrom(r io.Reader) (n int64, err error) {
	aw.mu.Lock()
	defer aw.mu.Unlock()
	return aw.buf.ReadFrom(r)
}

func (aw *AsyncWriter) initOnce() {
	aw.asyncWriteOnce.Do(func() {
		go aw.asyncWrite()
		if aw.opt.deadline > 0 {
			aw.deadlineTimer = time.NewTimer(aw.opt.deadline)
			go aw.deadline()
		}
	})
}

func (aw *AsyncWriter) asyncWrite() {
	for {
		b, ok := <-aw.queue
		if !ok {
			close(aw.asyncDone)
			return
		}

		if aw.err != nil {
			b.Free()
			continue
		}

		_, aw.err = b.WriteTo(aw.w)
		b.Free()
		if aw.err != nil {
			continue
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
				return
			}
		}
	}
}

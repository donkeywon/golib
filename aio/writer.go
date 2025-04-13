package aio

import (
	"io"
	"sync"
	"time"

	"github.com/donkeywon/golib/util/bytespool"
	"github.com/donkeywon/golib/util/iou"
)

type AsyncWriter struct {
	w              io.Writer
	err            error // TODO Flush和Close避免重复返回相同err
	opt            *option
	off            int
	buf            *bytespool.Bytes
	queue          chan *bytespool.Bytes
	closed         chan struct{}
	asyncDone      chan struct{}
	deadlineTimer  *time.Timer
	asyncWriteOnce sync.Once
	closeOnce      sync.Once
	mu             sync.Mutex
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
	aw.queue = make(chan *bytespool.Bytes, aw.opt.queueSize)
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
		aw.mu.Lock()

		aw.prepareBuf()
		nn = copy(aw.buf.B()[aw.off:], p)
		aw.off += nn
		p = p[nn:]
		n += nn

		if aw.off == aw.buf.Len() {
			err = aw.flushNoLock()
			if err != nil {
				aw.mu.Unlock()
				break
			}
		}

		aw.mu.Unlock()
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

	var (
		nn       int
		flushErr error
	)
	for {
		aw.mu.Lock()

		aw.prepareBuf()
		nn, err = iou.ReadFill(r, aw.buf.B()[aw.off:])
		aw.off += nn
		n += int64(nn)

		if err == nil && aw.off == aw.buf.Len() || err == io.EOF {
			flushErr = aw.flushNoLock()
			aw.mu.Unlock()
			if flushErr != nil || err == io.EOF {
				err = flushErr
				return
			}
			continue
		}

		aw.mu.Unlock()
		return
	}
}

func (aw *AsyncWriter) Flush() error {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	return aw.flushNoLock()
}

func (aw *AsyncWriter) flushNoLock() error {
	if aw.err != nil {
		return aw.err
	}

	if aw.buf == nil || aw.buf.Len() == 0 {
		return nil
	}

	aw.buf.Shrink(aw.off)
	aw.queue <- aw.buf
	aw.buf = nil
	if aw.deadlineTimer != nil {
		aw.deadlineTimer.Reset(aw.opt.deadline)
	}
	return nil
}

func (aw *AsyncWriter) prepareBuf() {
	if aw.buf != nil && aw.off < aw.buf.Len() {
		// not full
		return
	}

	aw.buf = bytespool.GetN(aw.opt.bufSize)
	aw.off = 0
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
	var nw int
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

		nw, aw.err = aw.w.Write(b.B())
		b.Free()
		if aw.err != nil {
			continue
		}
		if nw < b.Len() {
			aw.err = io.ErrShortWrite
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

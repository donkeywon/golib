package aio

import (
	"io"
	"sync"
	"time"
)

type loadOnceError struct {
	err    error
	loaded bool
}

func (e *loadOnceError) Has() bool {
	return e.err != nil
}

func (e *loadOnceError) Loaded() bool {
	return e.loaded
}

func (e *loadOnceError) Load() error {
	if e.loaded {
		return nil
	}
	e.loaded = true
	return e.err
}

func (e *loadOnceError) Err() error {
	e.loaded = true
	return e.err
}

func (e *loadOnceError) Store(err error) {
	e.err = err
}

type AsyncWriter struct {
	w              io.Writer
	err            loadOnceError
	opt            *option
	off            int
	buf            []byte
	bufChan        chan []byte
	queue          chan []byte
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
	aw.queue = make(chan []byte, aw.opt.queueSize)
	return aw
}

func (aw *AsyncWriter) Write(p []byte) (n int, err error) {
	select {
	case <-aw.closed:
		return 0, aw.err.Err()
	default:
	}

	aw.initOnce()

	var nn int
	for len(p) > 0 {
		if aw.err.Has() {
			return 0, aw.err.Err()
		}

		aw.mu.Lock()

		aw.prepareBuf()
		nn = copy(aw.buf[aw.off:], p)
		aw.off += nn
		p = p[nn:]
		n += nn

		if aw.off == len(aw.buf) {
			aw.flushNoLock()
		}

		aw.mu.Unlock()
	}

	return
}

func (aw *AsyncWriter) Close() error {
	var err error
	aw.closeOnce.Do(func() {
		close(aw.closed)

		aw.Flush()
		close(aw.queue)
		aw.initOnce()
		<-aw.asyncDone

		close(aw.bufChan)

		if aw.err.Has() {
			err = aw.err.Load()
		}
	})
	return err
}

func (aw *AsyncWriter) ReadFrom(r io.Reader) (n int64, err error) {
	select {
	case <-aw.closed:
		return 0, aw.err.Err()
	default:
	}

	aw.initOnce()

	var nn int
	for {
		if aw.err.Has() {
			return 0, aw.err.Err()
		}

		aw.mu.Lock()

		aw.prepareBuf()
		nn, err = r.Read(aw.buf[aw.off:])
		aw.off += nn
		n += int64(nn)

		if err == io.EOF || err == nil && aw.off == len(aw.buf) {
			aw.flushNoLock()
			aw.mu.Unlock()
			if err == io.EOF {
				err = nil
				return
			}
			continue
		}

		aw.mu.Unlock()
		if err == nil {
			continue
		}
		return
	}
}

func (aw *AsyncWriter) Flush() {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	aw.flushNoLock()
}

func (aw *AsyncWriter) flushNoLock() {
	if aw.err.Has() {
		return
	}

	if aw.buf == nil || len(aw.buf) == 0 {
		return
	}

	aw.buf = aw.buf[:aw.off]
	aw.queue <- aw.buf
	aw.buf = nil
	if aw.deadlineTimer != nil {
		aw.deadlineTimer.Reset(aw.opt.deadline)
	}
}

func (aw *AsyncWriter) prepareBuf() {
	if aw.buf != nil && aw.off < len(aw.buf) {
		// not full
		return
	}

	select {
	case aw.buf = <-aw.bufChan:
	default:
		aw.buf = make([]byte, aw.opt.bufSize)
	}
	aw.buf = aw.buf[:cap(aw.buf)]
	aw.off = 0
}

func (aw *AsyncWriter) initOnce() {
	aw.asyncWriteOnce.Do(func() {
		aw.bufChan = make(chan []byte, aw.opt.queueSize+2)

		go aw.asyncWrite()
		if aw.opt.deadline > 0 {
			aw.deadlineTimer = time.NewTimer(aw.opt.deadline)
			go aw.deadline()
		}
	})
}

func (aw *AsyncWriter) asyncWrite() {
	var (
		nw  int
		err error
	)
	for {
		b, ok := <-aw.queue
		if !ok {
			close(aw.asyncDone)
			return
		}

		if aw.err.Has() {
			aw.bufChan <- b
			continue
		}

		nw, err = aw.w.Write(b)
		aw.bufChan <- b
		if err != nil {
			aw.err.Store(err)
			continue
		}
		if nw < len(b) {
			aw.err.Store(io.ErrShortWrite)
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
			aw.Flush()
		}
	}
}

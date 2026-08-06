package aio

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type loadOnceError struct {
	mu     sync.RWMutex
	errs   []error
	loaded bool
}

func (e *loadOnceError) Has() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.errs) > 0
}

func (e *loadOnceError) LoadOnce() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.loaded {
		return nil
	}
	e.loaded = true
	return errors.Join(e.errs...)
}

func (e *loadOnceError) Load() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.errs) == 0 {
		return nil
	}

	e.loaded = true
	return errors.Join(e.errs...)
}

func (e *loadOnceError) Add(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.errs = append(e.errs, err)
}

type AsyncWriter struct {
	opt option
	w   io.Writer

	buf            []byte
	offset         int
	bufChan        chan []byte
	queue          chan []byte
	closeOnce      sync.Once
	closed         chan struct{}
	asyncWriteDone chan struct{}
	asyncWriteErr  loadOnceError
	initOnce       sync.Once
	initialized    atomic.Bool

	deadlineTimer *time.Timer
	mu            sync.Mutex // for deadlineFlush and Write+Flush and Close
}

func NewAsyncWriter(w io.Writer, opts ...Option) *AsyncWriter {
	aw := &AsyncWriter{
		w:              w,
		opt:            newOption(),
		closed:         make(chan struct{}),
		asyncWriteDone: make(chan struct{}),
	}
	for _, o := range opts {
		o.apply(&aw.opt)
	}
	aw.queue = make(chan []byte, aw.opt.queueSize)
	return aw
}

func (w *AsyncWriter) Write(p []byte) (n int, err error) {
	err = w.asyncWriteErr.Load()
	if err != nil {
		return
	}

	select {
	case <-w.closed:
		return 0, io.ErrClosedPipe
	default:
	}

	w.init()

	var nn int
	for len(p) > 0 {
		w.mu.Lock()

		w.prepareBuf()
		nn = copy(w.buf[w.offset:], p)
		w.offset += nn
		p = p[nn:]
		n += nn

		if w.offset < len(w.buf) {
			w.mu.Unlock()
			continue
		}

		err = w.flushNoLock()
		w.mu.Unlock()
		if err != nil {
			return
		}
	}
	return
}

func (w *AsyncWriter) Close() error {
	var err error
	w.closeOnce.Do(func() {
		err = w.Flush()

		close(w.closed)

		w.mu.Lock()
		close(w.queue)
		w.mu.Unlock()

		if w.initialized.Load() {
			<-w.asyncWriteDone
		}

		if err == nil {
			err = w.asyncWriteErr.LoadOnce()
		}
	})
	return err
}

func (w *AsyncWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushNoLock()
}

func (w *AsyncWriter) flushNoLock() error {
	err := w.asyncWriteErr.Load()
	if err != nil {
		return err
	}

	if w.buf == nil || w.offset == 0 {
		return nil
	}

	w.buf = w.buf[:w.offset]
	select {
	case w.queue <- w.buf:
		w.resetDeadline()
	case <-w.closed:
		return io.ErrClosedPipe
	}
	w.buf = nil
	return nil
}

func (w *AsyncWriter) prepareBuf() {
	if w.buf != nil && w.offset < len(w.buf) {
		// not full
		return
	}
	select {
	case w.buf = <-w.bufChan:
	default:
		w.buf = make([]byte, w.opt.bufSize)
	}
	w.buf = w.buf[:cap(w.buf)]
	w.offset = 0
}

func (w *AsyncWriter) init() {
	w.initOnce.Do(func() {
		w.bufChan = make(chan []byte, w.opt.queueSize+2)
		go w.asyncWrite()
		if w.opt.deadline > 0 {
			w.deadlineTimer = time.NewTimer(w.opt.deadline)
			go w.deadline()
		}
		w.initialized.Store(true)
	})
}

func (w *AsyncWriter) asyncWrite() {
	defer close(w.asyncWriteDone)

	for {
		buf, ok := <-w.queue
		if !ok {
			return
		}

		if w.asyncWriteErr.Has() {
			w.bufChan <- buf // len(bufChan)=len(queue)+2, no block
			continue
		}

		nw, err := w.w.Write(buf)
		w.bufChan <- buf // len(bufChan)=len(queue)+2, no block
		if err != nil {
			w.asyncWriteErr.Add(err)
		} else if nw < len(buf) {
			w.asyncWriteErr.Add(io.ErrShortWrite)
		}
	}
}

func (w *AsyncWriter) resetDeadline() {
	if w.deadlineTimer != nil {
		w.deadlineTimer.Reset(w.opt.deadline)
	}
}

func (w *AsyncWriter) deadline() {
	for {
		select {
		case <-w.closed:
			w.deadlineTimer.Stop()
			return
		case <-w.deadlineTimer.C:
			w.deadlineFlush()
			w.resetDeadline()
		}
	}
}

func (w *AsyncWriter) deadlineFlush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.opt.deadlineFlushMinSize > 0 && w.offset < w.opt.deadlineFlushMinSize {
		return
	}

	w.flushNoLock()
}

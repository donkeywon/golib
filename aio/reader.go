package aio

import (
	"errors"
	"io"
	"sync"
)

type AsyncReader struct {
	r   io.Reader
	opt *option

	buf   *buf
	queue chan *buf

	asyncReadOnce sync.Once
	closeOnce     sync.Once
	closed        chan struct{}

	err error
}

func NewAsyncReader(r io.Reader, opts ...Option) *AsyncReader {
	ar := &AsyncReader{
		r:      r,
		opt:    newOption(),
		closed: make(chan struct{}),
	}
	for _, o := range opts {
		o.apply(ar.opt)
	}
	ar.queue = make(chan *buf, ar.opt.queueSize)
	return ar
}

func (ar *AsyncReader) Read(p []byte) (int, error) {
	select {
	case <-ar.closed:
		return 0, ar.err
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	ar.asyncReadOnce.Do(func() {
		go ar.asyncRead()
	})

	var (
		n   int
		err error
		l   = len(p)
	)
	for n < l {
		err = ar.prepareBuf()
		if err != nil {
			break
		}

		n += ar.buf.writeToBytes(p)
	}
	return n, err
}

func (ar *AsyncReader) Close() error {
	ar.closeOnce.Do(func() {
		close(ar.closed)

		for b := range ar.queue {
			if b == nil {
				break
			}
			b.free()
		}
		if ar.buf != nil {
			ar.buf.free()
		}
	})
	return nil
}

func (ar *AsyncReader) WriteTo(w io.Writer) (n int64, err error) {
	ar.asyncReadOnce.Do(func() {
		go ar.asyncRead()
	})

	var nn int
	for {
		err = ar.prepareBuf()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return n, err
		}
		nn, err = ar.buf.writeTo(w)
		n += int64(nn)
		if err != nil {
			return n, err
		}
	}
}

func (ar *AsyncReader) prepareBuf() error {
	if ar.buf != nil && !ar.buf.isReadCompletely() {
		return nil
	}

	b, ok := <-ar.queue
	if !ok {
		return ar.err
	}

	if ar.buf != nil {
		ar.buf.free()
	}
	ar.buf = b
	return nil
}

func (ar *AsyncReader) asyncRead() {
	for {
		select {
		case <-ar.closed:
			return
		default:
		}

		b := newBuf(ar.opt.bufSize)
		nr, err := b.readFill(ar.r)
		if nr == 0 {
			b.free()
		} else {
			ar.queue <- b
		}

		if err != nil {
			ar.err = err
			close(ar.queue)
			return
		}
	}
}

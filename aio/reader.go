package aio

import (
	"errors"
	"io"
	"sync"

	"github.com/donkeywon/golib/util/buffer"
)

type AsyncReader struct {
	r             io.Reader
	err           error
	opt           *option
	buf           *buffer.FixedBuffer
	queue         chan *buffer.FixedBuffer
	closed        chan struct{}
	asyncReadOnce sync.Once
	closeOnce     sync.Once
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
	ar.queue = make(chan *buffer.FixedBuffer, ar.opt.queueSize)
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

	ar.initOnce()

	var (
		n   int
		nn  int
		err error
		l   = len(p)
	)
	for n < l {
		err = ar.prepareBuf()
		if err != nil {
			break
		}

		nn, _ = ar.buf.Read(p[n:])
		n += nn
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
			b.Free()
		}
		if ar.buf != nil {
			ar.buf.Free()
		}
	})
	return nil
}

func (ar *AsyncReader) WriteTo(w io.Writer) (n int64, err error) {
	ar.initOnce()

	var nn int64
	for {
		err = ar.prepareBuf()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return n, err
		}
		nn, err = ar.buf.WriteTo(w)
		n += nn
		if err != nil {
			return n, err
		}
	}
}

func (ar *AsyncReader) initOnce() {
	ar.asyncReadOnce.Do(func() {
		go ar.asyncRead()
	})
}

func (ar *AsyncReader) prepareBuf() error {
	if ar.buf != nil && ar.buf.HasRemaining() {
		return nil
	}

	b, ok := <-ar.queue
	if !ok {
		return ar.err
	}

	if ar.buf != nil {
		ar.buf.Free()
	}
	ar.buf = b
	return nil
}

func (ar *AsyncReader) asyncRead() {
	// TODO use LimitedReader ?
	for {
		select {
		case <-ar.closed:
			return
		default:
		}

		b := buffer.NewFixedBuffer(ar.opt.bufSize)
		nr, err := b.ReadFrom(ar.r)
		if nr == 0 {
			b.Free()
		} else {
			ar.queue <- b
		}

		if err == nil {
			err = io.EOF
		} else if err == buffer.ErrFull {
			// buf full and not EOF
			err = nil
		}

		if err != nil {
			ar.err = err
			close(ar.queue)
			return
		}
	}
}

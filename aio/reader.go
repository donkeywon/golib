package aio

import (
	"errors"
	"io"
	"sync"

	"github.com/donkeywon/golib/util/bytespool"
	"github.com/donkeywon/golib/util/iou"
)

type AsyncReader struct {
	r             io.Reader
	err           error
	opt           *option
	off           int
	buf           *bytespool.Bytes
	queue         chan *bytespool.Bytes
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
	ar.queue = make(chan *bytespool.Bytes, ar.opt.queueSize)
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

		nn = copy(p[n:], ar.buf.B()[ar.off:])
		ar.off += nn

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

	var (
		nn     int
		remain int
	)
	for {
		err = ar.prepareBuf()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return n, err
		}
		remain = ar.buf.Len() - ar.off
		nn, err = w.Write(ar.buf.B()[ar.off:])
		ar.off += nn
		n += int64(nn)
		if err != nil {
			return n, err
		}
		if nn < remain {
			return n, io.ErrShortWrite
		}
	}
}

func (ar *AsyncReader) initOnce() {
	ar.asyncReadOnce.Do(func() {
		go ar.asyncRead()
	})
}

func (ar *AsyncReader) prepareBuf() error {
	if ar.buf != nil && ar.off < ar.buf.Len() {
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
	ar.off = 0
	return nil
}

func (ar *AsyncReader) asyncRead() {
	for {
		select {
		case <-ar.closed:
			return
		default:
		}

		bs := bytespool.GetN(ar.opt.bufSize)
		nr, err := iou.ReadFill(ar.r, bs.B())
		if nr == 0 {
			bs.Free()
		} else {
			bs.Shrink(nr)
			ar.queue <- bs
		}

		if err != nil {
			ar.err = err
			close(ar.queue)
			return
		}
	}
}

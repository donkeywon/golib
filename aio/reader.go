package aio

import (
	"errors"
	"io"
	"sync"

	"github.com/donkeywon/golib/util/iou"
)

type AsyncReader struct {
	r             io.Reader
	err           error
	opt           *option
	off           int
	buf           []byte
	bufChan       chan []byte
	bufChanOnce   sync.Once
	queue         chan []byte
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
	ar.queue = make(chan []byte, ar.opt.queueSize)
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

		nn = copy(p[n:], ar.buf[ar.off:])
		ar.off += nn

		n += nn
	}
	return n, err
}

func (ar *AsyncReader) Close() error {
	ar.closeOnce.Do(func() { close(ar.closed) })
	ar.bufChanOnce.Do(func() { close(ar.bufChan) })
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
		remain = len(ar.buf) - ar.off
		nn, err = w.Write(ar.buf[ar.off:])
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
		ar.bufChan = make(chan []byte, ar.opt.queueSize+2)

		go ar.asyncRead()
	})
}

func (ar *AsyncReader) prepareBuf() error {
	if ar.buf != nil && ar.off < len(ar.buf) {
		return nil
	}

	b, ok := <-ar.queue
	if !ok {
		ar.bufChanOnce.Do(func() { close(ar.bufChan) })
		return ar.err
	}

	if ar.buf != nil {
		ar.bufChan <- ar.buf
	}
	ar.buf = b
	ar.off = 0
	return nil
}

func (ar *AsyncReader) asyncRead() {
	defer close(ar.queue)
	var b []byte
	for {
		select {
		case <-ar.closed:
			return
		default:
		}

		select {
		case b = <-ar.bufChan:
		default:
			b = make([]byte, ar.opt.bufSize)
		}

		b = b[:cap(b)]

		nr, err := iou.ReadFill(b, ar.r)
		b = b[:nr]
		ar.queue <- b

		if err != nil {
			ar.err = err
			return
		}
	}
}

package httpio

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
)

type writer struct {
	url     string
	timeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	closeOnce sync.Once

	opt *option
}

func NewWriter(ctx context.Context, timeout time.Duration, url string, opts ...Option) io.WriteCloser {
	if ctx == nil {
		ctx = context.Background()
	}

	w := &writer{
		url:     url,
		timeout: timeout,
		opt:     newOption(),
	}

	for _, o := range opts {
		o.apply(w.opt)
	}

	w.ctx, w.cancel = context.WithCancel(ctx)

	return w
}

func (w *writer) Write(p []byte) (int, error) {
	select {
	case <-w.ctx.Done():
		return 0, ErrAlreadyClosed
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	resp, err := w.put(httpc.WithBody(p))
	if err != nil {
		return 0, errs.Wrapf(err, "http put failed: %+v", resp)
	}

	return len(p), nil
}

func (w *writer) Close() error {
	w.closeOnce.Do(func() {
		w.cancel()
	})
	return nil
}

func (w *writer) ReadFrom(r io.Reader) (int64, error) {
	//var (
	//	total int64
	//	nw    int64
	//	err   error
	//)
	//
	//for {
	//	nw = 0
	//	r := io.LimitedReader{R: r, N: w.opt.partSize}
	//
	//}
	// TODO
	return 0, nil
}

func (w *writer) put(opts ...httpc.Option) (*http.Response, error) {
	allOpts := make([]httpc.Option, 0, len(opts)+len(w.opt.httpOptions))
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, w.opt.httpOptions...)
	return httpc.Put(w.ctx, w.timeout, w.url, allOpts...)
}

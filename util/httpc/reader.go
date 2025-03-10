package httpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpu"
)

var (
	ErrAlreadyClosed = errors.New("already closed")
)

type ReaderOption func(*Reader)

func (o ReaderOption) apply(r *Reader) {
	o(r)
}

func StartPos(pos int64) ReaderOption {
	return func(r *Reader) {
		r.pos = pos
	}
}

func WithReqOptions(opts ...Option) ReaderOption {
	return func(r *Reader) {
		r.reqOptions = append(r.reqOptions, opts...)
	}
}

func Retry(retry int) ReaderOption {
	return func(r *Reader) {
		r.retry = retry
	}
}

type Reader struct {
	url     string
	timeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	headOnce  sync.Once
	closeOnce sync.Once

	total        int64
	supportRange bool // TODO

	pos        int64
	reqOptions []Option
	retry      int
}

func NewReader(ctx context.Context, timeout time.Duration, url string, opts ...ReaderOption) *Reader {
	if ctx == nil {
		ctx = context.Background()
	}

	r := &Reader{
		url:     url,
		timeout: timeout,
	}

	r.ctx, r.cancel = context.WithCancel(ctx)

	for _, o := range opts {
		o.apply(r)
	}

	return r
}

func (r *Reader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, ErrAlreadyClosed
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	var headErr error
	r.headOnce.Do(func() {
		headErr = r.retryHead()
	})
	if headErr != nil {
		return 0, headErr
	}

	var nr int
	_, err := r.get(r.pos, int64(len(p)), ToBytes(&nr, p))
	r.pos += int64(nr)
	return nr, err
}

func (r *Reader) Close() error {
	r.closeOnce.Do(func() {
		r.cancel()
	})
	return nil
}

func (r *Reader) WriteTo(w io.Writer) (int64, error) {
	var (
		total int64
		nr    int64
		err   error
	)
	for {
		nr = 0
		_, er := retry.DoWithData(
			func() (*http.Response, error) {
				return r.get(r.pos, 8*1024*1024, ToWriter(&nr, w))
			},
			retry.Attempts(uint(r.retry)),
			retry.RetryIf(func(err error) bool {
				return err != nil && !errors.Is(err, io.EOF) && nr == 0
			}))
		if nr > 0 {
			r.pos += nr
		}

		if er != nil {
			if !errors.Is(er, io.EOF) {
				err = er
			}
			break
		}

		total += nr
		r.pos += nr
	}

	return total, err
}

func (r *Reader) retryHead() error {
	return retry.Do(func() error {
		return r.head()
	}, retry.Attempts(uint(r.retry)))
}

func (r *Reader) head() error {
	resp, err := Head(r.ctx, r.timeout, r.url, append(r.reqOptions, CheckStatusCode(http.StatusOK))...)
	if err != nil {
		return errs.Errorf("head failed, resp: %+v", resp)
	}

	r.total = resp.ContentLength
	if resp.Header.Get(httpu.HeaderAcceptRanges) == "bytes" && resp.ContentLength >= 0 {
		r.supportRange = true
	}

	return nil
}

func (r *Reader) get(start int64, n int64, opts ...Option) (*http.Response, error) {
	end := start + n - 1
	if end > r.total-1 {
		end = r.total - 1
	}
	ranges := fmt.Sprintf("bytes=%d-%d", start, end)

	resp, err := Get(r.ctx, r.timeout, r.url,
		append(opts, WithHeaders("Range", ranges)),
	)
	if err == nil && end == r.total-1 {
		err = io.EOF
	}
	return resp, err
}

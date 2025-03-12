package reader

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
	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/httpu"
)

var (
	ErrAlreadyClosed = errors.New("already closed")
)

type Option func(*Reader)

func (o Option) apply(r *Reader) {
	o(r)
}

func BeginPos(pos int64) Option {
	return func(r *Reader) {
		r.beginPos = pos
	}
}

func EndPos(pos int64) Option {
	return func(r *Reader) {
		r.endPos = pos
	}
}

func PartSize(s int64) Option {
	return func(r *Reader) {
		r.partSize = s
	}
}

func WithReqOptions(opts ...httpc.Option) Option {
	return func(r *Reader) {
		r.reqOptions = append(r.reqOptions, opts...)
	}
}

func Retry(retry int) Option {
	return func(r *Reader) {
		r.retry = retry
	}
}

type noCloseRespBody struct {
	respBody io.ReadCloser
}

func (b *noCloseRespBody) Read(p []byte) (int, error) {
	return b.respBody.Read(p)
}

func (b *noCloseRespBody) Close() error {
	return nil
}

type Reader struct {
	url     string
	timeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	headOnce  sync.Once
	closeOnce sync.Once

	total           int64
	supportRange    bool
	noRangeRespBody io.ReadCloser

	beginPos   int64
	endPos     int64
	partSize   int64
	reqOptions []httpc.Option
	retry      int
}

func New(ctx context.Context, timeout time.Duration, url string, opts ...Option) *Reader {
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

	if r.retry <= 0 {
		r.retry = 1
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

	err := r.doHeadOnce()
	if err != nil {
		return 0, err
	}

	if r.supportRange {
		var nr int
		_, err := r.getPart(r.beginPos, int64(len(p)), httpc.ToBytes(&nr, p))
		r.beginPos += int64(nr)
		if err != nil {
			return nr, err
		}
		if r.beginPos == r.total {
			return nr, io.EOF
		}
		return nr, nil
	}

	if r.noRangeRespBody == nil {
		r.noRangeRespBody, err = r.retryGetNoRange()
		if err != nil {
			return 0, err
		}
	}

	return r.noRangeRespBody.Read(p)
}

func (r *Reader) Close() error {
	var err error
	r.closeOnce.Do(func() {
		r.cancel()
		if r.noRangeRespBody != nil {
			err = r.noRangeRespBody.Close()
		}
	})
	return err
}

func (r *Reader) WriteTo(w io.Writer) (int64, error) {
	var (
		total int64
		nw    int64
		err   error
	)

	err = r.doHeadOnce()
	if err != nil {
		return 0, err
	}

	if !r.supportRange {
		_, err := retry.DoWithData(
			func() (*http.Response, error) {
				return r.get(httpc.ToWriter(&nw, w))
			},
			retry.Attempts(uint(r.retry)),
			retry.RetryIf(func(err error) bool {
				return err != nil && nw == 0
			}),
		)
		return nw, err
	}

	for {
		nw = 0
		_, err = retry.DoWithData(
			func() (*http.Response, error) {
				partSize := r.partSize
				if partSize == 0 {
					partSize = 8 * 1024 * 1024
				}
				return r.getPart(r.beginPos, partSize, httpc.ToWriter(&nw, w))
			},
			retry.Attempts(uint(r.retry)),
			retry.RetryIf(func(err error) bool {
				return err != nil && nw == 0
			}),
		)
		r.beginPos += nw
		total += nw
		if err != nil {
			break
		}
		if r.beginPos == r.total {
			break
		}
	}

	return total, err
}

func (r *Reader) doHeadOnce() error {
	var headErr error
	r.headOnce.Do(func() {
		headErr = r.retryHead()
	})
	return headErr
}

func (r *Reader) retryHead() error {
	return retry.Do(func() error {
		return r.head()
	}, retry.Attempts(uint(r.retry)))
}

func (r *Reader) head() error {
	resp, err := httpc.Head(r.ctx, r.timeout, r.url, r.reqOptions...)
	if err != nil {
		return errs.Errorf("head failed, resp: %+v", resp)
	}
	if resp.StatusCode >= 500 {
		return errs.Errorf("head response failed, resp: %+v", resp)
	}

	if resp.Header.Get(httpu.HeaderAcceptRanges) == "bytes" && resp.ContentLength >= 0 {
		r.total = resp.ContentLength
		r.supportRange = true
	}

	return nil
}

func (r *Reader) getPart(start int64, n int64, opts ...httpc.Option) (*http.Response, error) {
	end := min(start+n-1, r.total-1)
	ranges := fmt.Sprintf("bytes=%d-%d", start, end)

	allOpts := make([]httpc.Option, 0, len(r.reqOptions)+len(opts)+2)
	allOpts = append(allOpts, httpc.WithHeaders("Range", ranges), httpc.CheckStatusCode(http.StatusOK, http.StatusPartialContent))
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, r.reqOptions...)

	return httpc.Get(r.ctx, r.timeout, r.url, allOpts...)
}

func (r *Reader) retryGetNoRange() (io.ReadCloser, error) {
	var respBody io.ReadCloser
	_, err := retry.DoWithData(
		func() (*http.Response, error) {
			return r.get(httpc.RespOptionFunc(func(resp *http.Response) error {
				respBody = resp.Body
				resp.Body = &noCloseRespBody{
					respBody: resp.Body,
				}
				return nil
			}))
		},
		retry.Attempts(uint(r.retry)),
	)
	return respBody, err
}

func (r *Reader) get(opts ...httpc.Option) (*http.Response, error) {
	allOpts := make([]httpc.Option, 0, len(r.reqOptions)+len(opts)+1)
	allOpts = append(allOpts, httpc.CheckStatusCode(http.StatusOK))
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, r.reqOptions...)
	return httpc.Get(r.ctx, r.timeout, r.url, allOpts...)
}

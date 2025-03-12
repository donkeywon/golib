package httpio

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

	cfg        *Cfg
	pos        int64
	reqOptions []httpc.Option
}

func New(ctx context.Context, timeout time.Duration, url string, cfg *Cfg, opts ...httpc.Option) *Reader {
	if ctx == nil {
		ctx = context.Background()
	}

	r := &Reader{
		url:        url,
		timeout:    timeout,
		cfg:        cfg,
		reqOptions: opts,
	}

	r.ctx, r.cancel = context.WithCancel(ctx)

	if r.cfg.Retry <= 0 {
		r.cfg.Retry = 1
	}
	if r.cfg.PartSize <= 0 {
		r.cfg.PartSize = 8 * 1024 * 1024
	}
	if r.cfg.EndPos > 0 {
		r.total = r.cfg.EndPos
	}
	r.pos = r.cfg.BeginPos

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
		_, err := r.getPart(r.pos, int64(len(p)), httpc.ToBytes(&nr, p))
		r.pos += int64(nr)
		if err != nil {
			return nr, err
		}
		if r.pos == r.total {
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
			retry.Attempts(uint(r.cfg.Retry)),
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
				return r.getPart(r.pos, r.cfg.PartSize, httpc.ToWriter(&nw, w))
			},
			retry.Attempts(uint(r.cfg.Retry)),
			retry.RetryIf(func(err error) bool {
				return err != nil && nw == 0
			}),
		)
		r.pos += nw
		total += nw
		if err != nil {
			break
		}
		if r.pos == r.total {
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
	}, retry.Attempts(uint(r.cfg.Retry)))
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
		if r.total <= 0 {
			r.total = resp.ContentLength
		}
		r.supportRange = true
	}

	return nil
}

func (r *Reader) getPart(begin int64, n int64, opts ...httpc.Option) (*http.Response, error) {
	end := min(begin+n-1, r.total-1)
	ranges := fmt.Sprintf("bytes=%d-%d", begin, end)

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
		retry.Attempts(uint(r.cfg.Retry)),
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

func (r *Reader) Pos() int64 {
	return r.pos
}

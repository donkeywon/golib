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

type Reader struct {
	url     string
	timeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	headOnce  sync.Once
	closeOnce sync.Once

	len             int64
	end             int64
	supportRange    bool
	noRangeRespBody io.ReadCloser

	opt *option
}

func NewReader(ctx context.Context, timeout time.Duration, url string, opts ...Option) *Reader {
	if ctx == nil {
		ctx = context.Background()
	}

	r := &Reader{
		url:     url,
		timeout: timeout,
		opt:     newOption(),
		len:     -1,
	}

	for _, o := range opts {
		o.apply(r.opt)
	}

	r.ctx, r.cancel = context.WithCancel(ctx)

	if r.opt.limit > 0 {
		r.end = r.opt.offset + r.opt.limit
	}

	return r
}

func (r *Reader) Read(p []byte) (int, error) {
	nr, err := r.read(p, r.opt.offset)
	r.opt.offset += int64(nr)
	if err != nil {
		return nr, err
	}
	if r.reachEnd() {
		return nr, io.EOF
	}
	return nr, nil
}

func (r *Reader) ReadAt(p []byte, offset int64) (int, error) {
	return r.read(p, offset)
}

func (r *Reader) reachEnd() bool {
	return r.supportRange && r.opt.offset >= r.end
}

func (r *Reader) read(p []byte, offset int64) (int, error) {
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

	if !r.supportRange {
		if r.noRangeRespBody == nil {
			r.noRangeRespBody, err = r.retryGetNoRange()
			if err != nil {
				return 0, err
			}
		}

		return r.noRangeRespBody.Read(p)
	}

	var nr int
	_, err = r.getPart(offset, int64(len(p)), httpc.ToBytes(&nr, p))
	if err != nil {
		return 0, err
	}
	return nr, nil
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
	select {
	case <-r.ctx.Done():
		return 0, ErrAlreadyClosed
	default:
	}

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
		_, err = retry.DoWithData(
			func() (*http.Response, error) {
				return r.get(httpc.ToWriter(&nw, w))
			},
			retry.Attempts(uint(r.opt.retry)),
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
				return r.getPart(r.opt.offset, r.opt.partSize, httpc.ToWriter(&nw, w))
			},
			retry.Attempts(uint(r.opt.retry)),
			retry.RetryIf(func(err error) bool {
				return err != nil && nw == 0
			}),
		)
		r.opt.offset += nw
		total += nw
		if err != nil {
			break
		}
		if r.reachEnd() {
			break
		}
		if nw < r.opt.partSize {
			err = io.ErrShortWrite
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
	}, retry.Attempts(uint(r.opt.retry)))
}

func (r *Reader) head() error {
	resp, err := httpc.Head(r.ctx, r.timeout, r.url, r.opt.httpOptions...)
	if err != nil {
		return errs.Wrap(err, "head failed")
	}
	if resp.StatusCode >= 500 {
		return errs.Errorf("head response failed: %s", resp.Status)
	}

	if resp.Header.Get(httpu.HeaderAcceptRanges) == "bytes" && resp.ContentLength >= 0 {
		if r.opt.limit <= 0 {
			r.end = resp.ContentLength
		}
		r.supportRange = true
	}

	if r.end > 0 {
		r.len = r.end - r.opt.offset
	}

	return nil
}

func (r *Reader) getPart(offset int64, n int64, opts ...httpc.Option) (*http.Response, error) {
	end := min(offset+n-1, r.end-1)
	ranges := fmt.Sprintf("bytes=%d-%d", offset, end)

	allOpts := make([]httpc.Option, 0, len(r.opt.httpOptions)+len(opts)+2)
	allOpts = append(allOpts, httpc.WithHeaders("Range", ranges), httpc.CheckStatusCode(http.StatusOK, http.StatusPartialContent))
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, r.opt.httpOptions...)

	return httpc.Get(r.ctx, r.timeout, r.url, allOpts...)
}

func (r *Reader) retryGetNoRange() (io.ReadCloser, error) {
	var respBody io.ReadCloser
	_, err := retry.DoWithData(
		func() (*http.Response, error) {
			return r.get(httpc.RespOptionFunc(func(resp *http.Response) error {
				respBody = resp.Body
				resp.Body = io.NopCloser(resp.Body)
				return nil
			}))
		},
		retry.Attempts(uint(r.opt.retry)),
	)
	return respBody, err
}

func (r *Reader) get(opts ...httpc.Option) (*http.Response, error) {
	allOpts := make([]httpc.Option, 0, len(r.opt.httpOptions)+len(opts)+1)
	allOpts = append(allOpts, httpc.CheckStatusCode(http.StatusOK))
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, r.opt.httpOptions...)
	return httpc.Get(r.ctx, r.timeout, r.url, allOpts...)
}

// Offset is current offset of read.
func (r *Reader) Offset() int64 {
	return r.opt.offset
}

// Len returns content length need read, -1 means unknown.
func (r *Reader) Len() int64 {
	return r.len
}

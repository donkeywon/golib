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
	"github.com/donkeywon/golib/util/iou"
)

var (
	ErrRangeUnsupported = errors.New("range unsupported")
)

type respBodyReader struct {
	io.ReadCloser
	r *Reader
}

func (r *respBodyReader) Read(p []byte) (n int, err error) {
	n, err = r.ReadCloser.Read(p)
	r.r.offset += int64(n)
	return
}

type Reader struct {
	url     string
	timeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	headOnce  sync.Once
	closeOnce sync.Once

	offset       int64
	end          int64
	supportRange bool
	respBody     *respBodyReader

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
	}

	for _, o := range opts {
		o.apply(r.opt)
	}

	r.ctx, r.cancel = context.WithCancel(ctx)

	r.offset = r.opt.offset
	if r.opt.limit > 0 {
		r.end = r.opt.offset + r.opt.limit
	}

	return r
}

func (r *Reader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	err := r.init()
	if err != nil {
		return 0, err
	}

	if !r.supportRange {
		if r.respBody == nil {
			r.respBody, err = r.retryGetNoRange()
			if err != nil {
				return 0, err
			}
		}

		return r.respBody.Read(p)
	}

	return r.retryReadFromRemain(p)
}

func (r *Reader) ReadAt(p []byte, offset int64) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	err := r.init()
	if err != nil {
		return 0, err
	}
	if !r.supportRange {
		return 0, ErrRangeUnsupported
	}

	var nr int
	_, err = r.getPart(offset, int64(len(p)), httpc.ToBytes(&nr, p))
	return nr, err
}

func (r *Reader) Close() error {
	var err error
	r.closeOnce.Do(func() {
		r.cancel()
		if r.respBody != nil {
			err = r.respBody.Close()
		}
	})
	return err
}

func (r *Reader) WriteTo(w io.Writer) (int64, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
	}

	var (
		nw  int64
		err error
	)

	err = r.init()
	if err != nil {
		return 0, err
	}

	if !r.supportRange {
		_, err = r.get(httpc.ToWriter(&nw, w))
		return nw, err
	}

	return r.retryRemainWriteTo(w)
}

func (r *Reader) init() error {
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

	return nil
}

func (r *Reader) retryRemainWriteTo(w io.Writer) (int64, error) {
	var nw int64
	err := retry.Do(
		func() error {
			n, err := r.remainWriteTo(w)
			nw += n
			return err
		},
		retry.Attempts(uint(r.opt.retry)),
	)
	return nw, err
}

func (r *Reader) remainWriteTo(w io.Writer) (n int64, err error) {
	var respBody io.ReadCloser
	respBody, err = r.getRemain()
	if err != nil {
		return 0, err
	}
	defer respBody.Close()

	n, err = io.Copy(w, respBody)

	return n, err
}

func (r *Reader) retryReadFromRemain(p []byte) (int, error) {
	var nr int
	err := retry.Do(
		func() error {
			n, err := r.readFromRemain(p)
			p = p[n:]
			nr += n
			return err
		},
		retry.Attempts(uint(r.opt.retry)),
		retry.RetryIf(func(err error) bool {
			return err != nil && err != io.EOF
		}),
		retry.LastErrorOnly(true),
	)
	return nr, err
}

func (r *Reader) readFromRemain(p []byte) (n int, err error) {
	if r.respBody == nil {
		r.respBody, err = r.getRemain()
		if err != nil {
			r.respBody = nil
			return 0, err
		}
	}

	n, err = iou.ReadFill(p, r.respBody)
	if err != nil && err != io.EOF {
		r.respBody.Close()
		r.respBody = nil
	}
	return n, err
}

func (r *Reader) retryGetRemain() (io.ReadCloser, error) {
	return retry.DoWithData(
		func() (io.ReadCloser, error) {
			return r.getRemain()
		},
		retry.Attempts(uint(r.opt.retry)),
	)
}

func (r *Reader) getRemain() (*respBodyReader, error) {
	var respBody io.ReadCloser
	_, err := r.getPart(r.Offset(), r.end-r.Offset(), httpc.RespOptionFunc(func(resp *http.Response) error {
		respBody = resp.Body
		resp.Body = io.NopCloser(resp.Body)
		return nil
	}))
	return &respBodyReader{ReadCloser: respBody, r: r}, err
}

func (r *Reader) getPart(offset int64, n int64, opts ...httpc.Option) (*http.Response, error) {
	end := min(offset+n-1, r.end-1)
	ranges := fmt.Sprintf("bytes=%d-%d", offset, end)

	allOpts := make([]httpc.Option, 0, len(r.opt.httpOptions)+len(opts)+2)
	allOpts = append(allOpts, httpc.WithHeaders("Range", ranges), httpc.CheckStatusCode(http.StatusOK, http.StatusPartialContent))
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, r.opt.httpOptions...)

	return httpc.Get(r.ctx, 0, r.url, allOpts...)
}

func (r *Reader) retryGetNoRange() (*respBodyReader, error) {
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
	return &respBodyReader{ReadCloser: respBody, r: r}, err
}

func (r *Reader) get(opts ...httpc.Option) (*http.Response, error) {
	allOpts := make([]httpc.Option, 0, len(r.opt.httpOptions)+len(opts)+1)
	allOpts = append(allOpts, httpc.CheckStatusCode(http.StatusOK))
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, r.opt.httpOptions...)
	return httpc.Get(r.ctx, 0, r.url, allOpts...)
}

// Offset is current offset of read.
func (r *Reader) Offset() int64 {
	return r.offset
}

// Len returns content length need read, -1 means unknown.
func (r *Reader) Len() int64 {
	if r.end <= 0 {
		return -1
	}
	return r.end - r.offset
}

func (r *Reader) Size() int64 {
	if r.end <= 0 {
		return -1
	}
	return r.end - r.opt.offset
}

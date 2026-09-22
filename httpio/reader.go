package httpio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/donkeywon/golib/errs"
	"codeberg.org/donkeywon/golib/util/httpc"
	"codeberg.org/donkeywon/golib/util/httpu"
	"codeberg.org/donkeywon/golib/util/iou"
	"github.com/avast/retry-go/v4"
)

var (
	ErrRangeUnsupported = errors.New("range unsupported")
)

const (
	defaultResponseHeaderTimeout = time.Second * 15
)

type Reader struct {
	url string

	c          *http.Client
	withClient bool

	ctx    context.Context
	cancel context.CancelFunc

	initOnce  func() error
	closeOnce func() error

	offset       int64
	end          int64
	supportRange bool

	mu         sync.Mutex
	respBody   io.ReadCloser
	noRangeErr error

	opt *option
}

func NewReader(ctx context.Context, url string, opts ...Option) *Reader {
	if ctx == nil {
		panic("nil context")
	}

	r := &Reader{
		url: url,
		opt: newOption(),
	}
	r.initOnce = sync.OnceValue(r.retryHead)
	r.closeOnce = sync.OnceValue(func() error {
		if !r.withClient {
			defer r.c.CloseIdleConnections()
		}
		r.cancel()

		var err error
		r.mu.Lock()
		if r.respBody != nil {
			err = r.respBody.Close()
		}
		r.mu.Unlock()
		return err
	})

	for _, o := range opts {
		o(r.opt)
	}

	r.ctx, r.cancel = context.WithCancel(ctx)

	r.offset = r.opt.offset
	if r.opt.limit > 0 {
		r.end = r.opt.offset + r.opt.limit
	}

	if r.opt.client != nil {
		r.c = r.opt.client
		r.withClient = true
	} else {
		trans := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: defaultTransportDialContext(&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}),
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          1,
			MaxIdleConnsPerHost:   1,
			MaxConnsPerHost:       1,
			ResponseHeaderTimeout: defaultResponseHeaderTimeout,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		if r.opt.responseHeaderTimeout > 0 {
			trans.ResponseHeaderTimeout = r.opt.responseHeaderTimeout
		}
		r.c = &http.Client{
			Transport: trans,
		}
	}

	return r
}

func defaultTransportDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return dialer.DialContext
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

	err := r.initOnce()
	if err != nil {
		return 0, err
	}

	if !r.supportRange {
		return r.readNoRange(p)
	}

	return r.retryReadFromRemain(p)
}

func (r *Reader) readNoRange(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.noRangeErr != nil {
		return 0, r.noRangeErr
	}
	if r.opt.offset > 0 {
		return 0, ErrRangeUnsupported
	}

	if r.respBody == nil {
		var err error
		r.respBody, err = r.retryGetNoRange()
		if err != nil {
			return 0, err
		}
	}

	var remain int64
	if r.opt.limit > 0 {
		if remain = r.end - r.Offset(); remain <= 0 {
			return 0, io.EOF
		}
		p = p[:min(int64(len(p)), remain)]
	}

	n, err = r.respBody.Read(p)
	atomic.AddInt64(&r.offset, int64(n))
	if err != nil && err != io.EOF {
		r.respBody.Close()
		r.respBody = nil
		r.noRangeErr = err
	}
	return n, err
}

func (r *Reader) ReadAt(p []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, errors.New("httpio.Reader.ReadAt: negative offset")
	}
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	err := r.initOnce()
	if err != nil {
		return 0, err
	}
	if !r.supportRange {
		return 0, ErrRangeUnsupported
	}

	if offset >= r.end {
		return 0, io.EOF
	}

	var nr int
	err = retry.Do(
		func() error {
			return r.getPart(offset, int64(len(p)), httpc.ToBytes(&nr, p))
		},
		retry.Attempts(uint(r.opt.retry)),
		retry.LastErrorOnly(true),
		retry.RetryIf(retryable),
		retry.Context(r.ctx),
	)
	if err == nil && nr < len(p) {
		return nr, io.EOF
	}
	return nr, err
}

func (r *Reader) Close() error {
	return r.closeOnce()
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

	err = r.initOnce()
	if err != nil {
		return 0, err
	}

	if r.supportRange {
		return r.retryRemainWriteTo(w)
	}

	if r.opt.offset > 0 {
		return 0, ErrRangeUnsupported
	}

	err = r.get(httpc.RespOptionFunc(func(resp *http.Response) error {
		var cerr error
		if r.opt.limit > 0 {
			nw, cerr = io.CopyN(w, resp.Body, r.opt.limit)
			if cerr == io.EOF {
				cerr = nil
			}
		} else {
			nw, cerr = io.Copy(w, resp.Body)
		}
		return cerr
	}))
	atomic.AddInt64(&r.offset, nw)
	return nw, err
}

func (r *Reader) retryHead() error {
	return retry.Do(func() error {
		return r.head()
	}, retry.Attempts(uint(r.opt.retry)), retry.LastErrorOnly(true), retry.RetryIf(retryable), retry.Context(r.ctx))
}

func retryable(err error) bool {
	if err == nil {
		return false
	}

	if statusErr, ok := errors.AsType[httpc.StatusCodeError](err); ok {
		code := int(statusErr)
		return code >= http.StatusInternalServerError ||
			code == http.StatusRequestTimeout ||
			code == http.StatusTooManyRequests
	}

	return true
}

func retryableWithoutEOF(err error) bool {
	return err != io.EOF && retryable(err)
}

func (r *Reader) head() error {
	var (
		headerAcceptRange string
		contentLength     int64
	)
	err := httpc.DoWithClient(r.ctx, http.MethodHead, r.url, r.c,
		append(r.opt.httpOptions,
			httpc.CheckStatusCode(nil, nil, http.StatusOK),
			httpc.GetHeader(httpu.HeaderAcceptRanges, &headerAcceptRange),
			httpc.GetContentLength(&contentLength))...)
	if err != nil {
		return errs.Wrap(err, "head failed")
	}

	if headerAcceptRange == "bytes" && contentLength >= 0 {
		if r.opt.limit <= 0 {
			r.end = contentLength
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
			atomic.AddInt64(&r.offset, n)
			return err
		},
		retry.Attempts(uint(r.opt.retry)),
		retry.LastErrorOnly(true),
		retry.RetryIf(retryable),
		retry.Context(r.ctx),
	)
	return nw, err
}

func (r *Reader) remainWriteTo(w io.Writer) (n int64, err error) {
	respBody, err := r.getRemain()
	if errors.Is(err, io.EOF) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer respBody.Close()

	return io.Copy(w, respBody)
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
		retry.RetryIf(retryableWithoutEOF),
		retry.LastErrorOnly(true),
		retry.Context(r.ctx),
	)
	return nr, err
}

func (r *Reader) readFromRemain(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.respBody == nil {
		r.respBody, err = r.getRemain()
		if err != nil {
			r.respBody = nil
			return 0, err
		}
	}

	n, err = iou.ReadFill(p, r.respBody)
	atomic.AddInt64(&r.offset, int64(n))
	if err != nil && err != io.EOF {
		r.respBody.Close()
		r.respBody = nil
	}
	return n, err
}

func (r *Reader) getRemain() (io.ReadCloser, error) {
	var respBody io.ReadCloser
	n := r.Len()
	if n <= 0 {
		return nil, io.EOF
	}
	err := r.getPart(r.Offset(), n, httpc.DetachBody(&respBody))
	if err != nil {
		if respBody != nil {
			respBody.Close()
		}
		return nil, err
	}
	return respBody, nil
}

func (r *Reader) getPart(offset int64, n int64, opts ...httpc.Option) error {
	end := min(offset+n-1, r.end-1)
	ranges := fmt.Sprintf("bytes=%d-%d", offset, end)

	allOpts := make([]httpc.Option, 0, len(r.opt.httpOptions)+len(opts)+2)
	allOpts = append(allOpts, httpc.WithHeaders("Range", ranges), httpc.CheckStatusCode(nil, nil, http.StatusOK, http.StatusPartialContent))
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, r.opt.httpOptions...)

	return httpc.DoWithClient(r.ctx, http.MethodGet, r.url, r.c, allOpts...)
}

func (r *Reader) retryGetNoRange() (io.ReadCloser, error) {
	var respBody io.ReadCloser
	err := retry.Do(
		func() error {
			return r.get(httpc.DetachBody(&respBody))
		},
		retry.Attempts(uint(r.opt.retry)),
		retry.Context(r.ctx),
		retry.RetryIf(retryable),
		retry.OnRetry(func(attempt uint, err error) {
			if err != nil && respBody != nil {
				respBody.Close()
				respBody = nil
			}
		}),
	)
	if err != nil {
		if respBody != nil {
			respBody.Close()
		}
		return nil, err
	}
	return respBody, nil
}

func (r *Reader) get(opts ...httpc.Option) error {
	allOpts := make([]httpc.Option, 0, len(r.opt.httpOptions)+len(opts)+1)
	allOpts = append(allOpts, httpc.CheckStatusCode(nil, nil, http.StatusOK))
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, r.opt.httpOptions...)
	return httpc.DoWithClient(r.ctx, http.MethodGet, r.url, r.c, allOpts...)
}

// Offset is current offset of read.
func (r *Reader) Offset() int64 {
	return atomic.LoadInt64(&r.offset)
}

// Len returns content length need read, -1 means unknown.
func (r *Reader) Len() int64 {
	if r.end <= 0 {
		return -1
	}
	return r.end - r.Offset()
}

func (r *Reader) Size() int64 {
	if r.end <= 0 {
		return -1
	}
	return r.end - r.opt.offset
}

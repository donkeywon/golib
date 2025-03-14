package httpc

import (
	"context"
	"net/http"
	"time"

	"github.com/donkeywon/golib/errs"
)

func Get(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, timeout, http.MethodGet, url, opts...)
}

func Post(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, timeout, http.MethodPost, url, opts...)
}

func Head(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, timeout, http.MethodHead, url, opts...)
}

func Delete(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, timeout, http.MethodDelete, url, opts...)
}

func Put(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, timeout, http.MethodPut, url, opts...)
}

func Patch(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, timeout, http.MethodPatch, url, opts...)
}

func Connect(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, timeout, http.MethodConnect, url, opts...)
}

func Options(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, timeout, http.MethodOptions, url, opts...)
}

func Trace(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, timeout, http.MethodTrace, url, opts...)
}

func Do(ctx context.Context, timeout time.Duration, method string, url string, opts ...Option) (*http.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	r, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, errs.Wrap(err, "create http request failed")
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if h, ok := opt.(reqOption); ok {
			err = h.HandleReq(r)
			if err != nil {
				return nil, errs.Wrap(err, "handle http request failed")
			}
		}
	}

	var resp *http.Response
	resp, err = http.DefaultClient.Do(r)
	if err != nil {
		return resp, errs.Wrap(err, "do http request failed")
	}
	defer func() {
		// in case resp.Body was replaced, do not defer resp.Body.Close() directly
		resp.Body.Close()
	}()

	for _, opt := range opts {
		if h, ok := opt.(respOption); ok {
			err = h.HandleResp(resp)
			if err != nil {
				return resp, errs.Wrap(err, "handle http response failed")
			}
		}
	}

	return resp, nil
}

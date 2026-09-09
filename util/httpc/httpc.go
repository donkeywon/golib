package httpc

import (
	"context"
	"net/http"
	"time"

	"github.com/donkeywon/golib/errs"
)

func Get(ctx context.Context, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, http.MethodGet, url, opts...)
}

func GetTimeout(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return DoTimeout(ctx, timeout, http.MethodGet, url, opts...)
}

func Post(ctx context.Context, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, http.MethodPost, url, opts...)
}

func PostTimeout(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return DoTimeout(ctx, timeout, http.MethodPost, url, opts...)
}

func Head(ctx context.Context, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, http.MethodHead, url, opts...)
}

func HeadTimeout(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return DoTimeout(ctx, timeout, http.MethodHead, url, opts...)
}

func Delete(ctx context.Context, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, http.MethodDelete, url, opts...)
}

func DeleteTimeout(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return DoTimeout(ctx, timeout, http.MethodDelete, url, opts...)
}

func Put(ctx context.Context, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, http.MethodPut, url, opts...)
}

func PutTimeout(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return DoTimeout(ctx, timeout, http.MethodPut, url, opts...)
}

func Patch(ctx context.Context, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, http.MethodPatch, url, opts...)
}

func PatchTimeout(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return DoTimeout(ctx, timeout, http.MethodPatch, url, opts...)
}

func Connect(ctx context.Context, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, http.MethodConnect, url, opts...)
}

func ConnectTimeout(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return DoTimeout(ctx, timeout, http.MethodConnect, url, opts...)
}

func Options(ctx context.Context, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, http.MethodOptions, url, opts...)
}

func OptionsTimeout(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return DoTimeout(ctx, timeout, http.MethodOptions, url, opts...)
}

func Trace(ctx context.Context, url string, opts ...Option) (*http.Response, error) {
	return Do(ctx, http.MethodTrace, url, opts...)
}

func TraceTimeout(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error) {
	return DoTimeout(ctx, timeout, http.MethodTrace, url, opts...)
}

func Do(ctx context.Context, method string, url string, opts ...Option) (*http.Response, error) {
	return DoWithClient(ctx, method, url, http.DefaultClient, opts...)
}

func DoTimeout(ctx context.Context, timeout time.Duration, method string, url string, opts ...Option) (*http.Response, error) {
	return DoWithClientTimeout(ctx, timeout, method, url, http.DefaultClient, opts...)
}

func DoWithClient(ctx context.Context, method string, url string, client *http.Client, opts ...Option) (*http.Response, error) {
	return DoWithClientTimeout(ctx, 0, method, url, client, opts...)
}

func DoWithClientTimeout(ctx context.Context, timeout time.Duration, method string, url string, client *http.Client, opts ...Option) (*http.Response, error) {
	if ctx == nil {
		panic("nil context")
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	r, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, errs.Wrap(err, "create http request failed")
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		err = opt.HandleReq(r)
		if err != nil {
			return nil, errs.Wrap(err, "handle http request failed")
		}
	}

	var resp *http.Response
	resp, err = client.Do(r)
	if err != nil {
		return resp, errs.Wrap(err, "http request failed")
	}
	defer func() {
		// in case resp.Body was replaced, do not defer resp.Body.Close() directly
		resp.Body.Close()
	}()

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		err = opt.HandleResp(resp)
		if err != nil {
			return resp, errs.Wrap(err, "handle http response failed")
		}
	}

	return resp, nil
}

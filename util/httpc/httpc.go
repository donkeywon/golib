package httpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

var (
	ErrRespStatusCodeNotExpected = errors.New("response status code not expected")
)

func Get(url string, headersKV ...string) (*http.Response, error) {
	return DoRaw(http.MethodGet, url, nil, http.StatusOK, headersKV...)
}

func GetCtx(ctx context.Context, url string, headersKV ...string) (*http.Response, error) {
	return DoRawCtx(ctx, http.MethodGet, url, nil, http.StatusOK, headersKV...)
}

func GetTimeout(timeout time.Duration, url string, headersKV ...string) (*http.Response, error) {
	return DoRawTimeout(timeout, http.MethodGet, url, nil, http.StatusOK, headersKV...)
}

func G(url string, headersKV ...string) ([]byte, *http.Response, error) {
	return D(http.MethodGet, url, nil, http.StatusOK, headersKV...)
}

func Gctx(ctx context.Context, url string, headersKV ...string) ([]byte, *http.Response, error) {
	return Dctx(ctx, http.MethodGet, url, nil, http.StatusOK, headersKV...)
}

func Gtimeout(timeout time.Duration, url string, headersKV ...string) ([]byte, *http.Response, error) {
	return Dtimeout(timeout, http.MethodGet, url, nil, http.StatusOK, headersKV...)
}

func Post(url string, body []byte, expectedStatusCode int, headersKV ...string) (*http.Response, error) {
	return DoRaw(http.MethodPost, url, body, expectedStatusCode, headersKV...)
}

func PostCtx(ctx context.Context, url string, body []byte, expectedStatusCode int, headersKV ...string) (
	*http.Response, error,
) {
	return DoRawCtx(ctx, http.MethodPost, url, body, expectedStatusCode, headersKV...)
}

func PostTimeout(timeout time.Duration, url string, body []byte, expectedStatusCode int, headersKV ...string) (
	*http.Response, error,
) {
	return DoRawTimeout(timeout, http.MethodPost, url, body, expectedStatusCode, headersKV...)
}

func P(url string, body []byte, headersKV ...string) ([]byte, *http.Response, error) {
	return D(http.MethodPost, url, body, 0, headersKV...)
}

func Pctx(ctx context.Context, url string, body []byte, headersKV ...string) ([]byte, *http.Response, error) {
	return Dctx(ctx, http.MethodPost, url, body, 0, headersKV...)
}

func Ptimeout(timeout time.Duration, url string, body []byte, headersKV ...string) ([]byte, *http.Response, error) {
	return Dtimeout(timeout, http.MethodPost, url, body, 0, headersKV...)
}

func Put(url string, body []byte, expectedStatusCode int, headersKV ...string) (*http.Response, error) {
	return DoRaw(http.MethodPut, url, body, expectedStatusCode, headersKV...)
}

func PutCtx(ctx context.Context, url string, body []byte, expectedStatusCode int, headersKV ...string) (
	*http.Response, error,
) {
	return DoRawCtx(ctx, http.MethodPut, url, body, expectedStatusCode, headersKV...)
}

func PutTimeout(timeout time.Duration, url string, body []byte, expectedStatusCode int, headersKV ...string) (
	*http.Response, error,
) {
	return DoRawTimeout(timeout, http.MethodPut, url, body, expectedStatusCode, headersKV...)
}

func Pu(url string, body []byte, headersKV ...string) ([]byte, *http.Response, error) {
	return D(http.MethodPut, url, body, 0, headersKV...)
}

func PuCtx(ctx context.Context, url string, body []byte, headersKV ...string) ([]byte, *http.Response, error) {
	return Dctx(ctx, http.MethodPut, url, body, 0, headersKV...)
}

func PuTimeout(timeout time.Duration, url string, body []byte, headersKV ...string) ([]byte, *http.Response, error) {
	return Dtimeout(timeout, http.MethodPut, url, body, 0, headersKV...)
}

func D(method string, url string, body []byte, expectedStatusCode int, headersKV ...string) (
	[]byte, *http.Response, error,
) {
	return Dctx(context.Background(), method, url, body, expectedStatusCode, headersKV...)
}

func Dctx(ctx context.Context, method string, url string, body []byte, expectedStatusCode int, headersKV ...string) (
	[]byte, *http.Response, error,
) {
	return handleRespBody(DoRawCtx(ctx, method, url, body, expectedStatusCode, headersKV...))
}

func Dtimeout(timeout time.Duration, method string, url string, body []byte, checkCode int, headersKV ...string) (
	[]byte, *http.Response, error,
) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return Dctx(ctx, method, url, body, checkCode, headersKV...)
}

func DoRaw(method string, url string, body []byte, expectedStatusCode int, headersKV ...string) (
	*http.Response, error,
) {
	return DoRawCtx(context.Background(), method, url, body, expectedStatusCode, headersKV...)
}

func DoRawCtx(ctx context.Context, method string, url string, body []byte, checkCode int, headersKV ...string) (
	*http.Response, error,
) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	setHeaders(req, headersKV...)

	resp, err := Do(req)
	if err != nil {
		return resp, err
	}

	if checkCode > 0 && checkCode != resp.StatusCode {
		return resp, ErrRespStatusCodeNotExpected
	}

	return resp, nil
}

func DoRawTimeout(timeout time.Duration, method string, url string, body []byte, checkCode int, headersKV ...string) (
	*http.Response, error,
) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return DoRawCtx(ctx, method, url, body, checkCode, headersKV...)
}

func DoBody(req *http.Request) ([]byte, *http.Response, error) {
	return handleRespBody(Do(req))
}

func DoBodyCtx(ctx context.Context, req *http.Request) ([]byte, *http.Response, error) {
	return handleRespBody(DoCtx(ctx, req))
}

func Do(req *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func DoCtx(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	return Do(req)
}

func DoTimeout(req *http.Request, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return DoCtx(ctx, req)
}

func setHeaders(req *http.Request, headersKV ...string) {
	for i := 1; i < len(headersKV); i += 2 {
		req.Header.Set(headersKV[i-1], headersKV[i])
	}
}

func handleRespBody(resp *http.Response, requestErr error) ([]byte, *http.Response, error) {
	var (
		respBody    []byte
		readBodyErr error
	)
	if resp != nil {
		defer resp.Body.Close()
		respBody, readBodyErr = io.ReadAll(resp.Body)
	}
	return respBody, resp, errors.Join(requestErr, readBodyErr)
}

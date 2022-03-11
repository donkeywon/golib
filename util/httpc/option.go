package httpc

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpu"
	"github.com/donkeywon/golib/util/jsons"
)

var (
	ErrRespStatusCodeNotExpected = errors.New("response status code not expected")
)

// Option must implement reqOption or respOption.
type Option any

type reqOption interface {
	HandleReq(*http.Request) error
}

type respOption interface {
	HandleResp(*http.Response) error
}

type ReqOptionFunc func(r *http.Request) error

func (f ReqOptionFunc) HandleReq(r *http.Request) error {
	return f(r)
}

type RespOptionFunc func(r *http.Response) error

func (f RespOptionFunc) HandleResp(r *http.Response) error {
	return f(r)
}

func WithHeaders(headerKvs ...string) Option {
	return ReqOptionFunc(func(r *http.Request) error {
		for i := 1; i < len(headerKvs); i += 2 {
			r.Header.Add(headerKvs[i-1], headerKvs[i])
		}
		return nil
	})
}

func WithBody(body []byte) Option {
	return WithBodyReader(bytes.NewReader(body))
}

func WithBodyReader(reader io.Reader) Option {
	return ReqOptionFunc(func(r *http.Request) error {
		r.Body = io.NopCloser(reader)
		return nil
	})
}

func WithBodyJSON(v any) Option {
	return WithBodyMarshal(v, httpu.MIMEJSON, jsons.Marshal)
}

func WithBodyMarshal(v any, contentType string, marshal func(v any) ([]byte, error)) Option {
	return ReqOptionFunc(func(r *http.Request) error {
		bs, err := marshal(v)
		if err != nil {
			return errs.Wrap(err, "marshal request body failed")
		}
		r.Header.Set(httpu.HeaderContentType, contentType)
		r.Body = io.NopCloser(bytes.NewReader(bs))
		return nil
	})
}

func WithBodyForm(form url.Values) Option {
	return ReqOptionFunc(func(r *http.Request) error {
		if r == nil {
			return nil
		}
		r.Body = io.NopCloser(strings.NewReader(form.Encode()))
		r.Header.Set(httpu.HeaderContentType, httpu.MIMEPOSTForm)
		return nil
	})
}

func CheckStatusCode(statusCode ...int) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		if len(statusCode) == 0 {
			return nil
		}
		if slices.Contains(statusCode, resp.StatusCode) {
			return nil
		}
		return ErrRespStatusCodeNotExpected
	})
}

func ToBytes(n *int, b []byte) Option {
	return RespOptionFunc(func(r *http.Response) error {
		var err error
		if n == nil {
			_, err = r.Body.Read(b)
		} else {
			*n, err = r.Body.Read(b)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return errs.Wrap(err, "read response body failed")
		}
		return nil
	})
}

func ToBytesBuffer(n *int64, buf *bytes.Buffer) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		var err error
		if n == nil {
			_, err = io.Copy(buf, resp.Body)
		} else {
			*n, err = io.Copy(buf, resp.Body)
		}
		if err != nil {
			return errs.Wrap(err, "read response body failed")
		}
		return err
	})
}

func ToJSON(v any) Option {
	return ToAnyDecode(v, func(r io.Reader) httpu.Decoder { return jsons.NewDecoder(r) })
}

func ToAnyDecode(v any, newDecoder httpu.NewDecoder) Option {
	return RespOptionFunc(func(r *http.Response) error {
		err := newDecoder(r.Body).Decode(v)
		if err != nil && !errors.Is(err, io.EOF) {
			return errs.Wrap(err, "decode response body failed")
		}
		return nil
	})
}

func ToAnyUnmarshal(v any, unmarshaler func(bs []byte, v any) error) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			return errs.Wrap(err, "read response body failed")
		}
		err = unmarshaler(bs, v)
		if err != nil {
			return errs.Wrapf(err, "decode response body failed: %s", string(bs))
		}
		return nil
	})
}

func ToWriter(n *int64, w io.Writer) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		var err error
		if n == nil {
			_, err = io.Copy(w, resp.Body)
		} else {
			*n, err = io.Copy(w, resp.Body)
		}
		if err != nil {
			return errs.Wrap(err, "read response body to writer failed")
		}
		return nil
	})
}

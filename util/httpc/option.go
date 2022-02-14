package httpc

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
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
		for _, code := range statusCode {
			if resp.StatusCode == code {
				return nil
			}
		}
		return ErrRespStatusCodeNotExpected
	})
}

func ToBytes(n *int, b []byte) Option {
	return RespOptionFunc(func(r *http.Response) error {
		var err error
		*n, err = r.Body.Read(b)
		if err != nil {
			return errs.Wrap(err, "read response body failed")
		}
		return err
	})
}

func ToBytesBuffer(buf *bytes.Buffer) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		_, err := io.Copy(buf, resp.Body)
		if err != nil {
			return errs.Wrap(err, "read response body failed")
		}
		return err
	})
}

func ToJSON(v any) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		err := jsons.NewDecoder(resp.Body).Decode(v)
		if err != nil {
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

func ToWriter(w io.Writer) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		_, err := io.Copy(w, resp.Body)
		if err != nil {
			return errs.Wrap(err, "read response body to writer failed")
		}
		return nil
	})
}

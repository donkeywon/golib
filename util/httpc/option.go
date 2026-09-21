package httpc

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"codeberg.org/donkeywon/golib/errs"
	"codeberg.org/donkeywon/golib/util/conv"
	"codeberg.org/donkeywon/golib/util/httpu"
	"codeberg.org/donkeywon/golib/util/iou"
)

type Option interface {
	HandleReq(*http.Request) error
	HandleResp(*http.Response) error
}

type ReqOptionFunc func(r *http.Request) error

func (f ReqOptionFunc) HandleReq(r *http.Request) error {
	return f(r)
}

func (f ReqOptionFunc) HandleResp(_ *http.Response) error {
	return nil
}

type RespOptionFunc func(resp *http.Response) error

func (f RespOptionFunc) HandleReq(_ *http.Request) error {
	return nil
}

func (f RespOptionFunc) HandleResp(r *http.Response) error {
	return f(r)
}

func WithHeaders(headerKvs ...string) Option {
	if len(headerKvs)%2 == 1 {
		panic("odd header kvs")
	}
	return ReqOptionFunc(func(r *http.Request) error {
		for i := 1; i < len(headerKvs); i += 2 {
			r.Header.Add(headerKvs[i-1], headerKvs[i])
		}
		return nil
	})
}

func WithHeader(h http.Header) Option {
	return ReqOptionFunc(func(r *http.Request) error {
		for k, vs := range h {
			for _, v := range vs {
				r.Header.Add(k, v)
			}
		}
		return nil
	})
}

func WithHeaderMap(h map[string]string) Option {
	return ReqOptionFunc(func(r *http.Request) error {
		for k, v := range h {
			r.Header.Add(k, v)
		}
		return nil
	})
}

func WithBody(body []byte) Option {
	return WithBodyReader(bytes.NewReader(body))
}

type hasLen interface {
	Len() int
}

type hasLen2 interface {
	Len() int64
}

type hasSize interface {
	Size() int
}

type hasSize2 interface {
	Size() int64
}

func guessContentLength(r any) int64 {
	var l int64
	switch rt := r.(type) {
	case hasLen:
		l = int64(rt.Len())
	case hasLen2:
		l = rt.Len()
	case hasSize:
		l = int64(rt.Size())
	case hasSize2:
		l = rt.Size()
	default:
	}

	return l
}

func WithBodyReader(reader io.Reader) Option {
	return ReqOptionFunc(func(r *http.Request) error {
		if rc, ok := reader.(io.ReadCloser); ok {
			r.Body = rc
		} else {
			r.Body = io.NopCloser(reader)
		}

		l := guessContentLength(reader)
		if l > 0 {
			r.ContentLength = l
		}

		return nil
	})
}

func WithBodyJSON(v any) Option {
	return WithBodyMarshal(v, httpu.MIMEJSON, func(v any) ([]byte, error) { return json.Marshal(v) })
}

func WithBodyMarshal(v any, contentType string, marshal func(v any) ([]byte, error)) Option {
	return ReqOptionFunc(func(r *http.Request) error {
		bs, err := marshal(v)
		if err != nil {
			return errs.Wrap(err, "marshal request body failed")
		}
		r.Header.Set(httpu.HeaderContentType, contentType)
		r.Body = io.NopCloser(bytes.NewReader(bs))
		r.ContentLength = int64(len(bs))
		return nil
	})
}

func WithBodyForm(form url.Values) Option {
	return ReqOptionFunc(func(r *http.Request) error {
		if r == nil {
			return nil
		}
		s := form.Encode()
		r.Body = io.NopCloser(strings.NewReader(s))
		r.Header.Set(httpu.HeaderContentType, httpu.MIMEPOSTForm)
		r.ContentLength = int64(len(s))
		return nil
	})
}

func CheckStatusCode(checkFailedBodyWriter io.Writer, statusCode *int, expectedStatusCodes ...int) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		if statusCode != nil {
			*statusCode = resp.StatusCode
		}
		if len(expectedStatusCodes) == 0 {
			return nil
		}
		if slices.Contains(expectedStatusCodes, resp.StatusCode) {
			return nil
		}

		err := errs.Errorf("unexpected response status code: %d", resp.StatusCode)
		if checkFailedBodyWriter != nil {
			_, e := io.Copy(checkFailedBodyWriter, resp.Body)
			if e == io.EOF {
				e = nil
			}
			if e != nil {
				err = errors.Join(err, errs.Wrap(e, "read response body failed"))
			}
		}

		return err
	})
}

func CheckStatusCodeRange(checkFailedBodyWriter io.Writer, statusCode *int, min, max int) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		if statusCode != nil {
			*statusCode = resp.StatusCode
		}
		if resp.StatusCode >= min && resp.StatusCode <= max {
			return nil
		}

		err := errs.Errorf("unexpected response status code: %d", resp.StatusCode)
		if checkFailedBodyWriter != nil {
			_, e := io.Copy(checkFailedBodyWriter, resp.Body)
			if e == io.EOF {
				e = nil
			}
			if e != nil {
				err = errors.Join(err, errs.Wrap(e, "read response body failed"))
			}
		}

		return err
	})
}

func DetachBody(dst *io.ReadCloser) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		*dst = resp.Body
		resp.Body = http.NoBody
		return nil
	})
}

func GetHeader(k string, v *string) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		vv := resp.Header.Get(k)
		*v = vv
		return nil
	})
}

func GetContentLength(v *int64) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		*v = resp.ContentLength
		return nil
	})
}

func ToString(s *string) Option {
	return RespOptionFunc(func(resp *http.Response) error {
		var sb strings.Builder
		_, err := io.Copy(&sb, resp.Body)
		if err != nil {
			return errs.Wrap(err, "read response body failed")
		}
		*s = sb.String()
		return nil
	})
}

func ToBytes(n *int, b []byte) Option {
	return RespOptionFunc(func(r *http.Response) error {
		var err error
		if n == nil {
			_, err = iou.ReadFill(b, r.Body)
		} else {
			*n, err = iou.ReadFill(b, r.Body)
		}
		if err == io.EOF {
			err = nil
		}
		if err != nil {
			return errs.Wrap(err, "read response body failed")
		}
		return nil
	})
}

func ToJSON(v any) Option {
	return ToAnyDecode(v, func(r io.Reader, a any) error { return json.UnmarshalRead(r, a) })
}

func ToAnyDecode(v any, decode func(io.Reader, any) error) Option {
	return RespOptionFunc(func(r *http.Response) error {
		err := decode(r.Body, v)
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
			return errs.Wrapf(err, "unmarshal response body failed: %s", conv.Bytes2String(bs))
		}
		return nil
	})
}

func ToWriter(w io.Writer, n *int64) Option {
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

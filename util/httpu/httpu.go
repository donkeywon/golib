package httpu

import (
	"encoding/xml"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/conv"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/yamls"
)

const (
	HeaderContentType       = "Content-Type"
	HeaderContentLength     = "Content-Length"
	HeaderAcceptRanges      = "Accept-Ranges"
	HeaderAuthorization     = "Authorization"
	HeaderDate              = "Date"
	HeaderContentEncoding   = "Content-Encoding"
	HeaderContentLanguage   = "Content-Language"
	HeaderContentMD5        = "Content-MD5"
	HeaderIfModifiedSince   = "If-Modified-Since"
	HeaderIfMatch           = "If-Match"
	HeaderIfNoneMatch       = "If-None-Match"
	HeaderIfUnmodifiedSince = "If-Unmodified-Since"
	HeaderRange             = "Range"
	HeaderTransferEncoding  = "Transfer-Encoding"
	HeaderServer            = "Server"
	HeaderUserAgent         = "User-Agent"
	HeaderAccept            = "Accept"
	HeaderXForwardedFor     = "X-Forwarded-For"
	HeaderXRealIP           = "X-Real-Ip"

	MIMEHTML              = "text/html"
	MIMEHTMLUTF8          = "text/html; charset=utf-8"
	MIMEJSON              = "application/json"
	MIMEJSONUTF8          = "application/json; charset=utf-8"
	MIMEXML               = "application/xml"
	MIMEXML2              = "text/xml"
	MIMEPlain             = "text/plain"
	MIMEPlainUTF8         = "text/plain; charset=utf-8"
	MIMEPOSTForm          = "application/x-www-form-urlencoded"
	MIMEMultipartPOSTForm = "multipart/form-data"
	MIMEPROTOBUF          = "application/x-protobuf"
	MIMEMSGPACK           = "application/x-msgpack"
	MIMEMSGPACK2          = "application/msgpack"
	MIMEYAML              = "application/x-yaml"
	MIMEYAML2             = "application/yaml"
	MIMETOML              = "application/toml"
)

type Encoder interface {
	Encode(v any) error
}

type EncoderCreator func(w io.Writer) Encoder

type Decoder interface {
	Decode(v any) error
}

type DecoderCreator func(r io.Reader) Decoder

func RespBytes(w http.ResponseWriter, statusCode int, bs []byte, headersKV ...string) {
	setHeaders(w, headersKV...)
	w.WriteHeader(statusCode)
	_, err := w.Write(bs)
	if err != nil {
		panic(errs.Wrap(err, "write data to http response failed"))
	}
}

func RespString(w http.ResponseWriter, statusCode int, str string, headersKV ...string) {
	RespBytes(w, statusCode, conv.String2Bytes(str), headersKV...)
}

func RespReader(w http.ResponseWriter, statusCode int, r io.Reader, headersKV ...string) {
	setHeaders(w, headersKV...)
	w.WriteHeader(statusCode)
	_, err := io.Copy(w, r)
	if err != nil {
		panic(errs.Wrap(err, "copy reader to http response body failed"))
	}
}

func RespJSON(w http.ResponseWriter, statusCode int, data any, headersKV ...string) {
	RespEncoder(w, statusCode, data, MIMEJSON, func(w io.Writer) Encoder { return jsons.NewEncoder(w) }, headersKV...)
}

func RespYAML(w http.ResponseWriter, statusCode int, data any, headersKV ...string) {
	RespEncoder(w, statusCode, data, MIMEYAML, func(w io.Writer) Encoder { return yamls.NewEncoder(w) }, headersKV...)
}

func RespEncoder(w http.ResponseWriter, statusCode int, data any, mime string, newEncoder EncoderCreator, headersKV ...string) {
	setContentTypeHeader(w, mime)
	if data == nil {
		RespBytes(w, statusCode, nil, headersKV...)
		return
	}

	setHeaders(w, headersKV...)
	w.WriteHeader(statusCode)
	enc := newEncoder(w)
	err := enc.Encode(data)
	if err != nil {
		panic(errs.Wrap(err, "encode http response data failed"))
	}
}

func setHeaders(w http.ResponseWriter, headersKV ...string) {
	for i := 1; i < len(headersKV); i += 2 {
		w.Header().Add(headersKV[i-1], headersKV[i])
	}
}

func setContentTypeHeader(w http.ResponseWriter, t string) {
	ct := w.Header().Get(HeaderContentType)
	if ct == "" {
		w.Header().Set(HeaderContentType, t)
	}
}

func ReqToJSON(r *http.Request, obj any) error {
	err := jsons.NewDecoder(r.Body).Decode(obj)
	if err == io.EOF {
		return nil
	}
	return err
}

func ReqToXML(r *http.Request, obj any) error {
	err := xml.NewDecoder(r.Body).Decode(obj)
	if err == io.EOF {
		return nil
	}
	return err
}

func ReqToYAML(r *http.Request, obj any) error {
	err := yamls.NewDecoder(r.Body).Decode(obj)
	if err == io.EOF {
		return nil
	}
	return err
}

func GetRealRemoteIP(r *http.Request) string {
	if xff := r.Header.Get(HeaderXForwardedFor); xff != "" {
		for part := range strings.SplitSeq(xff, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if ip := net.ParseIP(part); ip != nil {
				return ip.String()
			}
		}
	}

	if xri := strings.TrimSpace(r.Header.Get(HeaderXRealIP)); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return ip.String()
		}
	}

	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

package httpu

import (
	"encoding/xml"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/conv"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/yamls"
	"github.com/pelletier/go-toml/v2"
	"google.golang.org/protobuf/proto"
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

type NewEncoder func(w io.Writer) Encoder

type Decoder interface {
	Decode(v any) error
}

type NewDecoder func(r io.Reader) Decoder

func RespOk(data any, w http.ResponseWriter, headersKV ...string) {
	Resp(w, http.StatusOK, data, headersKV...)
}

func RespFail(data any, w http.ResponseWriter, headersKV ...string) {
	Resp(w, http.StatusInternalServerError, data, headersKV...)
}

func Resp(w http.ResponseWriter, statusCode int, data any, headersKV ...string) {
	if data == nil {
		RespRaw(w, statusCode, nil, headersKV...)
		return
	}
	s, err := conv.ToString(data)
	if err != nil {
		RespRaw(w, http.StatusInternalServerError, conv.String2Bytes("convert response data to string failed: "+err.Error()), headersKV...)
		return
	}
	RespRaw(w, statusCode, conv.String2Bytes(s), headersKV...)
}

func RespRawOk(data []byte, w http.ResponseWriter, headersKV ...string) {
	RespRaw(w, http.StatusOK, data, headersKV...)
}

func RespRawFail(data []byte, w http.ResponseWriter, headersKV ...string) {
	RespRaw(w, http.StatusInternalServerError, data, headersKV...)
}

func RespRaw(w http.ResponseWriter, statusCode int, data []byte, headersKV ...string) {
	setHeaders(w, headersKV...)
	w.WriteHeader(statusCode)
	_, err := w.Write(data)
	if err != nil {
		panic(errs.Wrap(err, "http write data to response failed"))
	}
}

func RespReaderOk(w http.ResponseWriter, r io.Reader, headersKV ...string) {
	RespReader(w, http.StatusOK, r, headersKV...)
}

func RespReaderFail(w http.ResponseWriter, r io.Reader, headersKV ...string) {
	RespReader(w, http.StatusInternalServerError, r, headersKV...)
}

func RespReader(w http.ResponseWriter, statusCode int, r io.Reader, headersKV ...string) {
	setHeaders(w, headersKV...)
	w.WriteHeader(statusCode)
	_, err := io.Copy(w, r)
	if err != nil {
		panic(err)
	}
}

func RespJSONOk(w http.ResponseWriter, data any, headersKV ...string) {
	RespJSON(w, http.StatusOK, data, headersKV...)
}

func RespJSONFail(w http.ResponseWriter, data any, headersKV ...string) {
	RespJSON(w, http.StatusInternalServerError, data, headersKV...)
}

func RespJSON(w http.ResponseWriter, statusCode int, data any, headersKV ...string) {
	RespEncoder(w, statusCode, data, MIMEJSON, func(w io.Writer) Encoder { return jsons.NewEncoder(w) }, headersKV...)
}

func RespEncoder(w http.ResponseWriter, statusCode int, data any, mime string, newEncoder NewEncoder, headersKV ...string) {
	setContentTypeHeader(w, mime)
	if data == nil {
		RespRaw(w, statusCode, nil, headersKV...)
		return
	}

	setHeaders(w, headersKV...)
	w.WriteHeader(statusCode)
	enc := newEncoder(w)
	err := enc.Encode(data)
	if err != nil {
		panic(errs.Wrap(err, "encode http response data fail"))
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
	return jsons.NewDecoder(r.Body).Decode(obj)
}

func ReqToXML(r *http.Request, obj any) error {
	return xml.NewDecoder(r.Body).Decode(obj)
}

func ReqToYAML(r *http.Request, obj any) error {
	return yamls.NewDecoder(r.Body).Decode(obj)
}

func ReqToTOML(r *http.Request, obj any) error {
	return toml.NewDecoder(r.Body).Decode(obj)
}

func ReqToPB(r *http.Request, obj any) error {
	msg, ok := obj.(proto.Message)
	if !ok {
		return errors.New("obj is not proto.Message")
	}

	buf, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	return proto.Unmarshal(buf, msg)
}

func ReqTo(r *http.Request, obj any) error {
	var err error
	contentType := r.Header.Get(HeaderContentType)
	switch contentType {
	case MIMEJSON, MIMEJSONUTF8:
		err = ReqToJSON(r, obj)
	case MIMEXML, MIMEXML2:
		err = ReqToXML(r, obj)
	case MIMEYAML, MIMEYAML2:
		err = ReqToYAML(r, obj)
	case MIMETOML:
		err = ReqToTOML(r, obj)
	case MIMEPROTOBUF:
		err = ReqToPB(r, obj)
	default:
		err = ReqToJSON(r, obj)
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func GetRealRemoteIP(r *http.Request) string {
	remoteIP := ""
	// the default is the originating ip. but we try to find better options because this is almost
	// never the right IP
	if parts := strings.Split(r.RemoteAddr, ":"); len(parts) == 2 {
		remoteIP = parts[0]
	}
	// If we have a forwarded-for header, take the address from there
	if xff := strings.Trim(r.Header.Get("X-Forwarded-For"), ","); len(xff) > 0 {
		addrs := strings.Split(xff, ",")
		lastFwd := addrs[len(addrs)-1]
		if ip := net.ParseIP(lastFwd); ip != nil {
			remoteIP = ip.String()
		}
		// parse X-Real-Ip header
	} else if xri := r.Header.Get("X-Real-Ip"); len(xri) > 0 {
		if ip := net.ParseIP(xri); ip != nil {
			remoteIP = ip.String()
		}
	}

	return remoteIP
}

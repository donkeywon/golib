package httpu

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/conv"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/goccy/go-yaml"
	"github.com/pelletier/go-toml/v2"
	"google.golang.org/protobuf/proto"
)

const (
	HeaderContentType   = "Content-Type"
	HeaderContentLength = "Content-Length"

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

func RespOk(data interface{}, w http.ResponseWriter, headersKV ...string) {
	Resp(http.StatusOK, data, w, headersKV...)
}

func RespFail(data interface{}, w http.ResponseWriter, headersKV ...string) {
	Resp(http.StatusInternalServerError, data, w, headersKV...)
}

func Resp(statusCode int, data interface{}, w http.ResponseWriter, headersKV ...string) {
	if data == nil {
		RespRaw(statusCode, nil, w, headersKV...)
		return
	}
	s, err := conv.ToString(data)
	if err != nil {
		err = errs.Wrap(err, "convert response data to string fail")
		RespRaw(http.StatusInternalServerError, conv.String2Bytes(errs.ErrToStackString(err)), w, headersKV...)
		return
	}
	RespRaw(statusCode, conv.String2Bytes(s), w, headersKV...)
}

func RespRawOk(data []byte, w http.ResponseWriter, headersKV ...string) {
	RespRaw(http.StatusOK, data, w, headersKV...)
}

func RespRawFail(data []byte, w http.ResponseWriter, headersKV ...string) {
	RespRaw(http.StatusInternalServerError, data, w, headersKV...)
}

func RespRaw(statusCode int, data []byte, w http.ResponseWriter, headersKV ...string) {
	setHeaders(w, headersKV...)
	w.WriteHeader(statusCode)
	_, err := w.Write(data)
	if err != nil {
		panic(errs.Wrap(err, "http write data to response fail"))
	}
}

func RespHTMLOk(data []byte, w http.ResponseWriter, headersKV ...string) {
	RespHTML(http.StatusOK, data, w, headersKV...)
}

func RespHTMLFail(data []byte, w http.ResponseWriter, headersKV ...string) {
	RespHTML(http.StatusInternalServerError, data, w, headersKV...)
}

func RespHTML(statusCode int, data []byte, w http.ResponseWriter, headersKV ...string) {
	setContentTypeHeader(w, MIMEHTML)
	RespRaw(statusCode, data, w, headersKV...)
}

func RespJSONOk(data interface{}, w http.ResponseWriter, headersKV ...string) {
	RespJSON(http.StatusOK, data, w, headersKV...)
}

func RespJSONFail(data interface{}, w http.ResponseWriter, headersKV ...string) {
	RespJSON(http.StatusInternalServerError, data, w, headersKV...)
}

func RespJSON(statusCode int, data interface{}, w http.ResponseWriter, headersKV ...string) {
	setContentTypeHeader(w, MIMEJSON)
	if data == nil {
		RespRaw(statusCode, nil, w, headersKV...)
		return
	}

	bs, err := jsons.Marshal(data)
	if err != nil {
		panic(errs.Wrap(err, "data marshal to json fail"))
	}
	RespRaw(statusCode, bs, w, headersKV...)
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

func ReqToJSON(r *http.Request, obj interface{}) error {
	return jsons.NewDecoder(r.Body).Decode(obj)
}

func ReqToXML(r *http.Request, obj interface{}) error {
	return xml.NewDecoder(r.Body).Decode(obj)
}

func ReqToYAML(r *http.Request, obj interface{}) error {
	return yaml.NewDecoder(r.Body).Decode(obj)
}

func ReqToTOML(r *http.Request, obj interface{}) error {
	return toml.NewDecoder(r.Body).Decode(obj)
}

func ReqToPB(r *http.Request, obj interface{}) error {
	msg, ok := obj.(proto.Message)
	if !ok {
		return errors.New("obj is not proto.Message type")
	}

	buf, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	return proto.Unmarshal(buf, msg)
}

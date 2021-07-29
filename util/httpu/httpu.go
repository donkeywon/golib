package httpu

import (
	"net/http"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/convert"
	"github.com/donkeywon/golib/util/json"
)

const (
	HeaderContentType = "Content-Type"

	ContentTypeHTML     = "text/html"
	ContentTypeHTMLUTF8 = "text/html; charset=utf-8"
	ContentTypeJSON     = "application/json"
	ContentTypeJSONUTF8 = "application/json; charset=utf-8"
)

func RespOk(data interface{}, w http.ResponseWriter, headersKV ...string) {
	Resp(http.StatusOK, data, w, headersKV...)
}

func RespFail(data interface{}, w http.ResponseWriter, headersKV ...string) {
	Resp(http.StatusInternalServerError, data, w, headersKV...)
}

func Resp(statusCode int, data interface{}, w http.ResponseWriter, headersKV ...string) {
	s := convert.AnyToString(data)
	RespRaw(statusCode, convert.String2Bytes(s), w, headersKV...)
}

func RespRawOk(data []byte, w http.ResponseWriter, headersKV ...string) {
	RespRaw(http.StatusOK, data, w, headersKV...)
}

func RespRawFail(data []byte, w http.ResponseWriter, headersKV ...string) {
	RespRaw(http.StatusInternalServerError, data, w, headersKV...)
}

func RespRaw(statusCode int, data []byte, w http.ResponseWriter, headersKV ...string) {
	sendResponse(statusCode, data, w, headersKV...)
}

func RespHTMLOk(data []byte, w http.ResponseWriter, headersKV ...string) {
	RespHTML(http.StatusOK, data, w, headersKV...)
}

func RespHTMLFail(data []byte, w http.ResponseWriter, headersKV ...string) {
	RespHTML(http.StatusInternalServerError, data, w, headersKV...)
}

func RespHTML(statusCode int, data []byte, w http.ResponseWriter, headersKV ...string) {
	setContentTypeHeader(w, ContentTypeHTML)
	RespRaw(statusCode, data, w, headersKV...)
}

func RespJSONOk(data interface{}, w http.ResponseWriter, headersKV ...string) {
	RespJSON(http.StatusOK, data, w, headersKV...)
}

func RespJSONFail(data interface{}, w http.ResponseWriter, headersKV ...string) {
	RespJSON(http.StatusInternalServerError, data, w, headersKV...)
}

func RespJSON(statusCode int, data interface{}, w http.ResponseWriter, headersKV ...string) {
	setContentTypeHeader(w, ContentTypeJSON)
	bs, err := json.Marshal(data)
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

func sendResponse(statusCode int, data []byte, w http.ResponseWriter, headersKV ...string) {
	setHeaders(w, headersKV...)
	w.WriteHeader(statusCode)
	_, err := w.Write(data)
	if err != nil {
		panic(errs.Wrap(err, "http write data to response fail"))
	}
}

func setContentTypeHeader(w http.ResponseWriter, t string) {
	ct := w.Header().Get(HeaderContentType)
	if ct == "" {
		w.Header().Set(HeaderContentType, t)
	}
}

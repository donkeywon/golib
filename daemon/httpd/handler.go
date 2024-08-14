package httpd

import (
	"net/http"

	"github.com/donkeywon/golib/util/httpu"
)

type RawHandler func(http.ResponseWriter, *http.Request) []byte

func (rh RawHandler) Handle(w http.ResponseWriter, r *http.Request) []byte {
	return rh(w, r)
}

func (rh RawHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpu.RespRawOk(rh(w, r), w)
}

type APIHandler func(http.ResponseWriter, *http.Request) interface{}

func (ah APIHandler) Handle(w http.ResponseWriter, r *http.Request) interface{} {
	return ah(w, r)
}

func (ah APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpu.RespOk(ah(w, r), w)
}

type RESTHandler func(http.ResponseWriter, *http.Request) interface{}

func (rh RESTHandler) Handle(w http.ResponseWriter, r *http.Request) interface{} {
	return rh(w, r)
}

func (rh RESTHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpu.RespJSONOk(rh(w, r), w)
}

package httpd

import (
	"context"
	"net/http"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpu"
	"github.com/donkeywon/golib/util/v"
)

type RawHandler func(http.ResponseWriter, *http.Request) []byte

func (rh RawHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpu.RespRawOk(rh(w, r), w)
}

type APIHandler func(http.ResponseWriter, *http.Request) any

func (ah APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpu.RespOk(ah(w, r), w)
}

type RESTHandler[I any, O any] func(context.Context, I) (O, error)

func (rh RESTHandler[I, O]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		p := recover()
		if p != nil {
			httpu.RespJSONFail(w, &Resp{
				Code: RespCodeFail,
				Msg:  errs.PanicToErr(p).Error(),
			})
		}
	}()

	var i I
	err := httpu.ReqTo(r, &i)
	if err != nil {
		w.Header().Set(httpu.HeaderContentType, r.Header.Get(httpu.HeaderContentType))
		httpu.RespJSON(w, http.StatusBadRequest, &Resp{
			Code: RespCodeFail,
			Msg:  "parse request fail: " + err.Error(),
		})
		return
	}

	err = v.Struct(i)
	if err != nil {
		w.Header().Set(httpu.HeaderContentType, r.Header.Get(httpu.HeaderContentType))
		httpu.RespJSON(w, http.StatusBadRequest, &Resp{
			Code: RespCodeFail,
			Msg:  "validate request fail: " + err.Error(),
		})
		return
	}

	o, err := rh(r.Context(), i)
	if err != nil {
		httpu.RespJSONFail(w, &Resp{
			Code: RespCodeFail,
			Msg:  err.Error(),
		})
		return
	}

	httpu.RespJSONOk(w, &Resp{
		Code: RespCodeOk,
		Data: o,
	})
}

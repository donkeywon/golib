package httpd

import (
	"context"
	"github.com/donkeywon/golib/util/v"
	"net/http"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpu"
	"github.com/ggicci/httpin"
)

type RestReqCreator func() any

var _restReqCreatorMap = make(map[string]RestReqCreator)

type RawHandler func(http.ResponseWriter, *http.Request) []byte

func (rh RawHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpu.RespRawOk(rh(w, r), w)
}

type APIHandler func(http.ResponseWriter, *http.Request) any

func (ah APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpu.RespOk(ah(w, r), w)
}

type RESTHandler func(context.Context, any) (any, error)

func (rh RESTHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		p := recover()
		if p != nil {
			httpu.RespJSONFail(Resp{
				Code: RespCodeFail,
				Msg:  errs.ErrToStackString(errs.PanicToErr(p)),
			}, w)
		}
	}()

	var req any
	if reqCreator, ok := _restReqCreatorMap[r.Pattern]; ok {
		req = reqCreator()
		err := httpin.DecodeTo(r, req)
		if err != nil {
			httpu.RespJSONFail(Resp{
				Code: RespCodeFail,
				Msg:  "parse request fail: " + err.Error(),
			}, w)
			return
		}

		err = v.Struct(req)
		if err != nil {
			httpu.RespJSONFail(Resp{
				Code: RespCodeFail,
				Msg:  "validate request fail: " + err.Error(),
			}, w)
		}
	}

	result, err := rh(r.Context(), req)
	if err != nil {
		httpu.RespJSONFail(Resp{
			Code: RespCodeFail,
			Msg:  err.Error(),
		}, w)
		return
	}

	httpu.RespJSONOk(Resp{
		Code: RespCodeOk,
		Data: result,
	}, w)
}

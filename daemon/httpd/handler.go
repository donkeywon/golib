package httpd

import (
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

type validateStruct struct {
	Any any `validate:"dive"`
}

type RESTHandler[I any, O any] func(http.ResponseWriter, *http.Request, I) (O, error)

func (rh RESTHandler[I, O]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		p := recover()
		if p != nil {
			httpu.RespJSONFail(w, &Resp{
				Code: RespCodeFail,
				Msg:  errs.ErrToStackString(errs.PanicToErr(p)),
			})
		}
	}()

	var i I
	err := httpu.ReqTo(r, &i)
	if err != nil {
		httpu.RespJSON(w, http.StatusBadRequest, &Resp{
			Code: RespCodeFail,
			Msg:  errs.ErrToStackString(errs.Wrap(err, "parse request failed")),
		})
		return
	}

	if !isNil(i) {
		err = v.Struct(&validateStruct{i})
		if err != nil {
			httpu.RespJSON(w, http.StatusBadRequest, &Resp{
				Code: RespCodeFail,
				Msg:  errs.ErrToStackString(errs.Wrap(err, "invalid request")),
			})
			return
		}
	}

	o, err := rh(w, r, i)
	if err != nil {
		httpu.RespJSONFail(w, &Resp{
			Code: RespCodeFail,
			Msg:  errs.ErrToStackString(errs.Wrap(err, "handle request failed")),
		})
		return
	}

	httpu.RespJSONOk(w, &Resp{
		Code: RespCodeOk,
		Data: o,
	})
}

func isNil(a any) bool {
	return a == nil
}
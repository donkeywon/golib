package httpd

import (
	"net/http"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpu"
)

type RespCode int

const (
	RespCodeOk   RespCode = 0
	RespCodeFail RespCode = 1
)

type Resp struct {
	Code RespCode `json:"code"`
	Msg  string   `json:"msg"`
	Data any      `json:"data"`
}

func RestRecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			e := recover()
			if e == nil {
				return
			}

			resp := &Resp{
				Code: RespCodeFail,
				Msg:  errs.PanicToErr(e).Error(),
			}
			httpu.RespJSONFail(w, resp)
		}()

		next.ServeHTTP(w, r)
	})
}

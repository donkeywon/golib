package httpd

import (
	"errors"
	"fmt"
	"net/http"
	"plugin"
	"time"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/conv"
	"github.com/donkeywon/golib/util/httpu"
)

const DaemonTypeHttpd boot.DaemonType = "httpd"

type MiddlewareFunc func(http.Handler) http.Handler

func init() {
	_h.RegisterMiddleware(logAndRecoverMiddleware)
}

var _h = &Httpd{
	Runner: runner.Create(string(DaemonTypeHttpd)),
	mux:    http.NewServeMux(),
}

type Httpd struct {
	runner.Runner
	plugin.Plugin
	*Cfg

	s           *http.Server
	mux         *http.ServeMux
	middlewares []MiddlewareFunc
}

func newHTTPServer(cfg *Cfg) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

func New() *Httpd {
	return _h
}

func (h *Httpd) Start() error {
	h.s = newHTTPServer(h.Cfg)
	h.setMux()
	return h.s.ListenAndServe()
}

func (h *Httpd) Stop() error {
	return h.s.Close()
}

func (h *Httpd) Type() interface{} {
	return DaemonTypeHttpd
}

func (h *Httpd) GetCfg() interface{} {
	return h.Cfg
}

func (h *Httpd) AppendError(err ...error) {
	for _, e := range err {
		if !errors.Is(e, http.ErrServerClosed) {
			h.Runner.AppendError(e)
		}
	}
}

func (h *Httpd) RegisterMiddleware(mf ...MiddlewareFunc) {
	h.middlewares = append(h.middlewares, mf...)
}

func (h *Httpd) setMux() {
	h.s.Handler = h.mux
}

func (h *Httpd) buildHandlerChain(next http.Handler) http.Handler {
	handler := next
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		handler = h.middlewares[i](handler)
	}
	return handler
}

func logFields(r *http.Request, w *recordResponseWriter, startTs int64, endTs int64) []interface{} {
	return []interface{}{
		"status", w.statusCode,
		"uri", r.RequestURI,
		"remote", r.RemoteAddr,
		"req_method", r.Method,
		"req_body_size", r.ContentLength,
		"resp_body_size", w.nw,
		"cost", fmt.Sprintf("%.6fms", float64(endTs-startTs)/float64(time.Millisecond)),
	}
}

// RegisterMiddleware must called before Handle func below
func RegisterMiddleware(mf ...MiddlewareFunc) {
	_h.RegisterMiddleware(mf...)
}

func Handle(pattern string, handler http.Handler) {
	_h.mux.Handle(pattern, _h.buildHandlerChain(handler))
}

func HandleFunc(pattern string, handler http.HandlerFunc) {
	_h.mux.HandleFunc(pattern, _h.buildHandlerChain(handler).ServeHTTP)
}

func HandleRaw(pattern string, handler RawHandler) {
	_h.mux.Handle(pattern, _h.buildHandlerChain(handler))
}

func HandleAPI(pattern string, handler APIHandler) {
	_h.mux.Handle(pattern, _h.buildHandlerChain(handler))
}

func HandleREST(pattern string, handler RESTHandler) {
	_h.mux.Handle(pattern, _h.buildHandlerChain(handler))
}

func logAndRecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w = newWriteOnceRecordResponseWriter(w)

		start := time.Now().UnixNano()
		defer func() {
			end := time.Now().UnixNano()

			e := recover()
			if e != nil {
				err := errs.PanicToErr(e)
				_h.Error("handle req fail, panic occurred", err, logFields(r, w.(*recordResponseWriter), start, end)...)
				errStr := errs.ErrToStackString(err)
				httpu.RespRaw(http.StatusInternalServerError, conv.String2Bytes(errStr), w)
			} else {
				_h.Info("handle req", logFields(r, w.(*recordResponseWriter), start, end)...)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

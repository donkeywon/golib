package httpd

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/conv"
	"github.com/donkeywon/golib/util/httpu"
)

func init() {
	D().RegMiddleware(D().logAndRecoverMiddleware)
}

const DaemonTypeHTTPd boot.DaemonType = "httpd"

type MiddlewareFunc func(http.Handler) http.Handler

var _h = New()

type HTTPd struct {
	runner.Runner
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

func D() *HTTPd {
	return _h
}

func New() *HTTPd {
	return &HTTPd{
		Runner: runner.Create(string(DaemonTypeHTTPd)),
		mux:    http.NewServeMux(),
	}
}

func (h *HTTPd) Start() error {
	h.s = newHTTPServer(h.Cfg)
	h.setMux()
	return h.s.ListenAndServe()
}

func (h *HTTPd) Stop() error {
	return h.s.Close()
}

func (h *HTTPd) Type() any {
	return DaemonTypeHTTPd
}

func (h *HTTPd) GetCfg() any {
	return h.Cfg
}

func (h *HTTPd) AppendError(err ...error) {
	for _, e := range err {
		if !errors.Is(e, http.ErrServerClosed) {
			h.Runner.AppendError(e)
		}
	}
}

func (h *HTTPd) RegMiddleware(mf ...MiddlewareFunc) {
	h.middlewares = append(h.middlewares, mf...)
}

func (h *HTTPd) setMux() {
	h.s.Handler = h.mux
}

func (h *HTTPd) buildHandlerChain(next http.Handler) http.Handler {
	handler := next
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		handler = h.middlewares[i](handler)
	}
	return handler
}

func logFields(r *http.Request, w *recordResponseWriter, startTs int64, endTs int64) []any {
	return []any{
		"status", w.statusCode,
		"uri", r.RequestURI,
		"remote", r.RemoteAddr,
		"req_method", r.Method,
		"req_body_size", r.ContentLength,
		"resp_body_size", w.nw,
		"cost", fmt.Sprintf("%.6fms", float64(endTs-startTs)/float64(time.Millisecond)),
	}
}

func (h *HTTPd) Handle(pattern string, handler http.Handler) {
	h.mux.Handle(pattern, h.buildHandlerChain(handler))
}

func (h *HTTPd) HandleFunc(pattern string, handler http.HandlerFunc) {
	h.mux.HandleFunc(pattern, h.buildHandlerChain(handler).ServeHTTP)
}

func (h *HTTPd) HandleRaw(pattern string, handler RawHandler) {
	h.mux.Handle(pattern, h.buildHandlerChain(handler))
}

func (h *HTTPd) HandleAPI(pattern string, handler APIHandler) {
	h.mux.Handle(pattern, h.buildHandlerChain(handler))
}

func (h *HTTPd) HandleREST(pattern string, handler RESTHandler, reqBodyCreator RestReqCreator) {
	if reqBodyCreator != nil {
		_restReqCreatorMap[pattern] = reqBodyCreator
	}
	h.mux.Handle(pattern, h.buildHandlerChain(handler))
}

func (h *HTTPd) logAndRecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w = newWriteOnceRecordResponseWriter(w)

		start := time.Now().UnixNano()
		defer func() {
			end := time.Now().UnixNano()

			e := recover()
			if e != nil {
				err := errs.PanicToErr(e)
				h.Error("handle req fail, panic occurred", err, logFields(r, w.(*recordResponseWriter), start, end)...)
				errStr := errs.ErrToStackString(err)
				httpu.RespRaw(http.StatusInternalServerError, conv.String2Bytes(errStr), w)
			} else {
				h.Debug("handle req done", logFields(r, w.(*recordResponseWriter), start, end)...)
			}
		}()

		h.Debug("begin handling req",
			"uri", r.RequestURI,
			"remote", r.RemoteAddr,
			"req_method", r.Method,
			"req_body_size", r.ContentLength)
		next.ServeHTTP(w, r)
	})
}

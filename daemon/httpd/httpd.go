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

type HTTPD interface {
	boot.Daemon
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler http.HandlerFunc)
	RegMiddleware(mid ...MiddlewareFunc)
	Mux() *http.ServeMux
	Cfg() Cfg
}

func init() {
	D.RegMiddleware(logAndRecoverMiddleware)
}

const DaemonTypeHTTPd boot.DaemonType = "httpd"

type MiddlewareFunc func(http.Handler) http.Handler

var (
	D HTTPD = New()
)

type httpd struct {
	runner.Runner

	cfg         *Cfg
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

func New() HTTPD {
	return &httpd{
		Runner: runner.Create(string(DaemonTypeHTTPd)),
		mux:    http.NewServeMux(),
	}
}

func (h *httpd) Start() error {
	h.s = newHTTPServer(h.cfg)
	h.setMuxToHTTPHandler()
	return h.s.ListenAndServe()
}

func (h *httpd) Stop() error {
	return h.s.Close()
}

func (h *httpd) AppendError(err ...error) {
	for _, e := range err {
		if !errors.Is(e, http.ErrServerClosed) {
			h.Runner.AppendError(e)
		}
	}
}

func (h *httpd) RegMiddleware(mf ...MiddlewareFunc) {
	h.middlewares = append(h.middlewares, mf...)
}

func (h *httpd) Mux() *http.ServeMux {
	return h.mux
}

func (h *httpd) Cfg() Cfg {
	return *h.cfg
}

func (h *httpd) SetCfg(cfg any) {
	h.cfg = cfg.(*Cfg)
}

func (h *httpd) setMuxToHTTPHandler() {
	h.s.Handler = h.mux
}

func (h *httpd) buildHandlerChain(next http.Handler) http.Handler {
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

func (h *httpd) Handle(pattern string, handler http.Handler) {
	h.mux.Handle(pattern, h.buildHandlerChain(handler))
}

func (h *httpd) HandleFunc(pattern string, handler http.HandlerFunc) {
	h.mux.HandleFunc(pattern, h.buildHandlerChain(handler).ServeHTTP)
}

func HandleREST[I any, O any](pattern string, handler RESTHandler[I, O]) {
	D.Handle(pattern, handler)
}

func logAndRecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w = newWriteOnceRecordResponseWriter(w)

		D.Debug("begin handle req",
			"uri", r.RequestURI,
			"remote", r.RemoteAddr,
			"req_method", r.Method,
			"req_body_size", r.ContentLength)

		start := time.Now().UnixNano()
		defer func() {
			end := time.Now().UnixNano()

			e := recover()
			if e != nil {
				err := errs.PanicToErr(e)
				D.Error("panic on handle req", err, logFields(r, w.(*recordResponseWriter), start, end)...)
				httpu.RespRaw(w, http.StatusInternalServerError, conv.String2Bytes(err.Error()))
			} else {
				D.Debug("end handle req", logFields(r, w.(*recordResponseWriter), start, end)...)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

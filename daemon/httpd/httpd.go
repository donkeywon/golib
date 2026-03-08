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

const DaemonTypeHTTPd boot.DaemonType = "httpd"

type httpd struct {
	runner.Runner

	cfg         *Cfg
	s           *http.Server
	mux         *http.ServeMux
	middlewares []func(http.Handler) http.Handler
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

func New() boot.Daemon {
	h := &httpd{
		Runner: runner.Create(string(DaemonTypeHTTPd)),
		mux:    http.NewServeMux(),
	}
	h.Use(h.logAndRecoverMiddleware)
	return h
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

func (h *httpd) Use(mf ...func(http.Handler) http.Handler) {
	h.middlewares = append(h.middlewares, mf...)
}

func (h *httpd) Mux() *http.ServeMux {
	return h.mux
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

func (h *httpd) Handle(pattern string, handler http.Handler) {
	h.mux.Handle(pattern, h.buildHandlerChain(handler))
}

func (h *httpd) HandleFunc(pattern string, handler http.HandlerFunc) {
	h.mux.HandleFunc(pattern, h.buildHandlerChain(handler).ServeHTTP)
}

func (h *httpd) logAndRecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rpw := &recordResponseWriter{
			ResponseWriter: w,
			nw:             -1,
			statusCode:     http.StatusOK,
		}

		h.Debug("begin handle req",
			"uri", r.RequestURI,
			"remote", r.RemoteAddr,
			"req_method", r.Method,
			"req_body_size", r.ContentLength)

		start := time.Now().UnixNano()
		defer func() {
			end := time.Now().UnixNano()

			nw := rpw.nw
			if nw < 0 {
				nw = 0
			}

			e := recover()
			if e != nil {
				err := errs.PanicToErr(e)
				h.Error("panic on handle req", err,
					"status", rpw.statusCode,
					"uri", r.RequestURI,
					"remote", r.RemoteAddr,
					"req_method", r.Method,
					"req_body_size", r.ContentLength,
					"resp_body_size", nw,
					"cost", fmt.Sprintf("%.6fms", float64(end-start)/float64(time.Millisecond)))
				httpu.RespBytes(w, http.StatusInternalServerError, conv.String2Bytes(err.Error()))
			} else {
				h.Debug("end handle req",
					"status", rpw.statusCode,
					"uri", r.RequestURI,
					"remote", r.RemoteAddr,
					"req_method", r.Method,
					"req_body_size", r.ContentLength,
					"resp_body_size", nw,
					"cost", fmt.Sprintf("%.6fms", float64(end-start)/float64(time.Millisecond)))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

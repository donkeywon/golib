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
)

const SvcTypeHttpd boot.SvcType = "httpd"

var _h = &Httpd{
	Runner: runner.NewBase("httpd"),
	mux:    http.NewServeMux(),
}

type Httpd struct {
	runner.Runner
	plugin.Plugin
	*Cfg

	s   *http.Server
	mux *http.ServeMux
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
	h.setHandler()
	return h.s.ListenAndServe()
}

func (h *Httpd) Stop() error {
	return h.s.Close()
}

func (h *Httpd) Type() interface{} {
	return SvcTypeHttpd
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

func (h *Httpd) setHandler() {
	h.s.Handler = h.mux
}

func (h *Httpd) logHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UnixNano()
		defer func() {
			end := time.Now().UnixNano()

			err := recover()
			if err != nil {
				h.Error("handle req fail", errs.Errorf("panic: %+v", err),
					"url", r.RequestURI,
					"remote", r.RemoteAddr,
					"cost", fmt.Sprintf("%.6fms", float64(end-start)/float64(time.Millisecond)))
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				h.Info("handle req",
					"url", r.RequestURI,
					"remote", r.RemoteAddr,
					"cost", fmt.Sprintf("%.6fms", float64(end-start)/float64(time.Millisecond)))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func Handle(pattern string, handler http.Handler) {
	_h.mux.Handle(pattern, _h.logHandler(handler))
}

func HandleFunc(pattern string, handler http.HandlerFunc) {
	_h.mux.HandleFunc(pattern, _h.logHandler(handler).ServeHTTP)
}

func HandleAPI(pattern string, handler APIHandler) {
	_h.mux.Handle(pattern, _h.logHandler(handler))
}

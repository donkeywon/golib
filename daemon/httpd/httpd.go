package httpd

import (
	"errors"
	"net/http"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/runner"
)

const DaemonTypeHTTPd boot.DaemonType = "httpd"

var _ HTTPd = (*httpd)(nil)

type HTTPd interface {
	boot.Daemon
	SetRouter(Router)
	Server() *http.Server
	Use(...func(http.Handler) http.Handler)
	Handle(string, http.Handler)
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
}

type Router interface {
	http.Handler
	Handle(string, http.Handler)
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
}

type httpd struct {
	runner.Runner

	cfg *Cfg
	s   *http.Server

	r           Router
	patterns    []string
	handlers    []http.Handler
	middlewares []func(http.Handler) http.Handler
}

func New() boot.Daemon {
	return &httpd{
		Runner: runner.Create(string(DaemonTypeHTTPd)),
	}
}

func (h *httpd) Init() error {
	if h.r == nil {
		h.r = http.NewServeMux()
	}

	for i := range h.patterns {
		h.r.Handle(h.patterns[i], h.buildHandlerChain(h.handlers[i]))
	}

	h.s.Handler = h.r
	return h.Runner.Init()
}

func (h *httpd) Start() error {
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

func (h *httpd) Handle(pattern string, handler http.Handler) {
	h.patterns = append(h.patterns, pattern)
	h.handlers = append(h.handlers, handler)
}

func (h *httpd) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	h.patterns = append(h.patterns, pattern)
	h.handlers = append(h.handlers, http.HandlerFunc(handler))
}

func (h *httpd) SetCfg(cfg any) {
	h.cfg = cfg.(*Cfg)
	h.s = h.cfg.buildHTTPServer()
}

func (h *httpd) SetRouter(r Router) {
	h.r = r
}

func (h *httpd) Server() *http.Server {
	return h.s
}

func (h *httpd) buildHandlerChain(next http.Handler) http.Handler {
	handler := next
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		handler = h.middlewares[i](handler)
	}
	return handler
}

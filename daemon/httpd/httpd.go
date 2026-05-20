package httpd

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/logs"
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
	Logger() *slog.Logger
}

type Router interface {
	http.Handler
	Handle(string, http.Handler)
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
}

type httpd struct {
	runner.Base

	cfg *Cfg
	s   *http.Server

	l           *slog.Logger
	r           Router
	patterns    []string
	handlers    []http.Handler
	middlewares []func(http.Handler) http.Handler
}

func New() boot.Daemon {
	return &httpd{}
}

func (h *httpd) Init(ctx context.Context) error {
	h.l = logs.FromCtx(ctx)
	if h.r == nil {
		h.r = http.NewServeMux()
	}

	for i := range h.patterns {
		h.r.Handle(h.patterns[i], h.buildHandlerChain(h.handlers[i]))
	}

	h.s.Handler = h.r
	return nil
}

func (h *httpd) Start(_ context.Context) error {
	err := h.s.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (h *httpd) Stop(ctx context.Context) error {
	return h.s.Shutdown(ctx)
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

func (h *httpd) Logger() *slog.Logger {
	return h.l
}

func (h *httpd) buildHandlerChain(next http.Handler) http.Handler {
	handler := next
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		handler = h.middlewares[i](handler)
	}
	return handler
}

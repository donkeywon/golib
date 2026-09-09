package httpd

import (
	"context"
	"errors"
	"net/http"

	"github.com/donkeywon/golib/boot"
	"github.com/rs/zerolog"
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
	Logger() *zerolog.Logger
}

type Router interface {
	http.Handler
	Handle(string, http.Handler)
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
}

type httpd struct {
	cfg Cfg
	s   *http.Server

	l           *zerolog.Logger
	r           Router
	patterns    []string
	handlers    []http.Handler
	middlewares []func(http.Handler) http.Handler
}

func New() boot.Daemon {
	return &httpd{}
}

func (h *httpd) Init(ctx context.Context) error {
	h.l = zerolog.Ctx(ctx)
	if h.r == nil {
		h.r = http.NewServeMux()
	}

	for i := range h.patterns {
		h.r.Handle(h.patterns[i], h.buildHandlerChain(h.handlers[i]))
	}

	h.s.Handler = h.r
	return nil
}

func (h *httpd) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		err := h.s.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err, ok := <-errCh:
		if !ok {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), h.cfg.ShutdownTimeout)
		defer cancel()
		return errors.Join(ctx.Err(), h.s.Shutdown(shutdownCtx))
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

func (h *httpd) SetCfg(cfg Cfg) {
	h.cfg = cfg
	h.s = h.cfg.buildHTTPServer()
}

func (h *httpd) SetRouter(r Router) {
	h.r = r
}

func (h *httpd) Server() *http.Server {
	return h.s
}

func (h *httpd) Logger() *zerolog.Logger {
	return h.l
}

func (h *httpd) buildHandlerChain(next http.Handler) http.Handler {
	handler := next
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		handler = h.middlewares[i](handler)
	}
	return handler
}

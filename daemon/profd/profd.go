package profd

import (
	"context"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"

	"github.com/arl/statsviz"
	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/daemon/httpd"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpu"
	"github.com/felixge/fgprof"
	"github.com/google/gops/agent"
	"github.com/maruel/panicparse/v2/stack/webstack"
	"github.com/rs/zerolog"
)

const DaemonTypeProfd boot.DaemonType = "profd"

var _ Profd = (*profd)(nil)

type Profd interface {
	boot.Daemon
	SetAllowedIPsGetter(func() map[string]struct{})
}

type profd struct {
	*zerolog.Logger

	cfg Cfg
	ctx context.Context

	allowedIPsGetter func() map[string]struct{}

	prettystackMu       sync.Mutex
	prettystackLastTime time.Time

	statsvizServer *statsviz.Server

	httpd httpd.HTTPd
}

func New() boot.Daemon {
	return &profd{}
}

func (p *profd) Init(ctx context.Context) error {
	p.Logger = zerolog.Ctx(ctx)

	if p.cfg.EnableStatsViz || p.cfg.EnableWebProf || p.cfg.EnableWebPrettyTrace {
		p.httpd = boot.Get[httpd.HTTPd](httpd.DaemonTypeHTTPd)
	}

	var err error
	if p.cfg.EnableStatsViz {
		p.statsvizServer, err = statsviz.NewServer()
		if err != nil {
			return errs.Wrap(err, "init statsviz failed")
		} else {
			p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/statsviz/", p.midSecure(p.statsvizServer.Index()))
			p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/statsviz/ws", p.midSecure(p.statsvizServer.Ws()))
		}
	}

	if p.cfg.EnableWebProf {
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/pprof/", p.midSecure(pprof.Index))
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/pprof/cmdline", p.midSecure(pprof.Cmdline))
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/pprof/profile", p.midSecure(pprof.Profile))
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/pprof/symbol", p.midSecure(pprof.Symbol))
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/pprof/trace", p.midSecure(pprof.Trace))
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/pprof/allocs", p.midSecure(pprof.Handler("allocs").ServeHTTP))
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/pprof/block", p.midSecure(pprof.Handler("block").ServeHTTP))
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/pprof/goroutine", p.midSecure(pprof.Handler("goroutine").ServeHTTP))
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/pprof/heap", p.midSecure(pprof.Handler("heap").ServeHTTP))
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/pprof/mutex", p.midSecure(pprof.Handler("mutex").ServeHTTP))
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/pprof/threadcreate", p.midSecure(pprof.Handler("threadcreate").ServeHTTP))

		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/fgprof", fgprof.Handler())
	}

	if p.cfg.EnableWebPrettyTrace {
		p.httpd.Handle(p.cfg.HTTPURLPrefix+"/debug/prettytrace", p.midSecure(p.prettytrace))
	}

	if p.cfg.EnableGoPs {
		err := agent.Listen(agent.Options{Addr: p.cfg.GoPsAddr})
		if err != nil {
			return errs.Wrap(err, "init gops agent failed")
		}
	}

	return nil
}

func (p *profd) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (p *profd) SetCfg(cfg Cfg) {
	p.cfg = cfg
}

func (p *profd) Cfg() Cfg {
	return p.cfg
}

func (p *profd) SetAllowedIPsGetter(allowedIPsGetter func() map[string]struct{}) {
	p.allowedIPsGetter = allowedIPsGetter
}

func (p *profd) midSecure(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !p.secure(w, r) {
			return
		}

		f(w, r)
	}
}

func (p *profd) secure(w http.ResponseWriter, r *http.Request) bool {
	if !p.ipAllowed(w, r) {
		return false
	}

	if !p.auth(w, r) {
		return false
	}

	return true
}

func (p *profd) ipAllowed(w http.ResponseWriter, r *http.Request) bool {
	if p.allowedIPsGetter == nil {
		return true
	}

	ips := p.allowedIPsGetter()
	if len(ips) == 0 {
		return true
	}

	remoteIP := httpu.GetRealRemoteIP(r)
	if _, exist := ips[remoteIP]; exist {
		return true
	}

	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	return false
}

func (p *profd) auth(w http.ResponseWriter, r *http.Request) bool {
	if p.cfg.WebAuthUser == "" && p.cfg.WebAuthPwd == "" {
		return true
	}
	user, pass, ok := r.BasicAuth()
	if ok && user == p.cfg.WebAuthUser && pass == p.cfg.WebAuthPwd {
		return true
	}

	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted", charset="UTF-8"`)
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	return false
}

func (p *profd) prettytrace(w http.ResponseWriter, r *http.Request) {
	p.prettystackMu.Lock()
	defer p.prettystackMu.Unlock()

	// Throttle requests.
	if time.Since(p.prettystackLastTime) < time.Second {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	webstack.SnapshotHandler(w, r)
	p.prettystackLastTime = time.Now()
}

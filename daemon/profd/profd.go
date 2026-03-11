package profd

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"strconv"
	"sync"
	"time"

	"github.com/arl/statsviz"
	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/daemon/httpd"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/httpu"
	"github.com/donkeywon/golib/util/prof"
	"github.com/google/gops/agent"
	"github.com/maruel/panicparse/v2/stack/webstack"
)

const DaemonTypeProfd boot.DaemonType = "profd"

var _ Profd = (*profd)(nil)

type Profd interface {
	boot.Daemon
	SetAllowedIPsGetter(func() map[string]struct{})
}

type profd struct {
	runner.Runner

	cfg *Cfg

	allowedIPsGetter func() map[string]struct{}

	prettystackMu       sync.Mutex
	prettystackLastTime time.Time

	statsvizServer *statsviz.Server

	httpd httpd.HTTPd
}

func New() boot.Daemon {
	return &profd{
		Runner: runner.Create(string(DaemonTypeProfd)),
	}
}

func (p *profd) Init() error {
	p.httpd = boot.Get[httpd.HTTPd](boot.DaemonType("httpd"))

	var err error
	if p.cfg.EnableStartupProfiling {
		filepath, done, err := prof.Start(p.cfg.StartupProfilingMode, p.cfg.ProfOutputDir, p.cfg.StartupProfilingSec)
		if err != nil {
			if !p.cfg.SkipStartupErr {
				return errs.Wrap(err, "startup profiling failed")
			}
			p.Error("startup profiling failed", err,
				"mode", p.cfg.StartupProfilingMode,
				"duration", fmt.Sprintf("%ds", p.cfg.StartupProfilingSec),
				"filepath", filepath)
		} else {
			p.Info("startup profiling",
				"mode", p.cfg.StartupProfilingMode,
				"duration", fmt.Sprintf("%ds", p.cfg.StartupProfilingSec),
				"filepath", filepath)
			go func() {
				select {
				case <-done:
					p.Info("startup profiling done",
						"mode", p.cfg.StartupProfilingMode,
						"duration", fmt.Sprintf("%ds", p.cfg.StartupProfilingSec),
						"filepath", filepath)
				case <-p.Stopping():
					return
				}
			}()
		}
	}

	if p.cfg.EnableStatsViz {
		p.statsvizServer, err = statsviz.NewServer()
		if err != nil {
			if !p.cfg.SkipStartupErr {
				return errs.Wrap(err, "init statsviz failed")
			}
			p.Error("init statsviz failed", err)
		} else {
			p.httpd.Handle(p.cfg.Prefix+"/debug/statsviz/", p.midSecure(p.statsvizServer.Index()))
			p.httpd.Handle(p.cfg.Prefix+"/debug/statsviz/ws", p.midSecure(p.statsvizServer.Ws()))
		}
	}

	if p.cfg.EnableHTTPProf {
		p.httpd.Handle(p.cfg.Prefix+"/debug/prof/start/{mode}", http.HandlerFunc(p.startProf))
		p.httpd.Handle(p.cfg.Prefix+"/debug/prof/stop", http.HandlerFunc(p.stopProf))
	}

	if p.cfg.EnableWebProf {
		p.httpd.Handle(p.cfg.Prefix+"/debug/pprof/", p.midSecure(pprof.Index))
		p.httpd.Handle(p.cfg.Prefix+"/debug/pprof/cmdline", p.midSecure(pprof.Cmdline))
		p.httpd.Handle(p.cfg.Prefix+"/debug/pprof/profile", p.midSecure(pprof.Profile))
		p.httpd.Handle(p.cfg.Prefix+"/debug/pprof/symbol", p.midSecure(pprof.Symbol))
		p.httpd.Handle(p.cfg.Prefix+"/debug/pprof/trace", p.midSecure(pprof.Trace))
		p.httpd.Handle(p.cfg.Prefix+"/debug/pprof/allocs", p.midSecure(pprof.Handler("allocs").ServeHTTP))
		p.httpd.Handle(p.cfg.Prefix+"/debug/pprof/block", p.midSecure(pprof.Handler("block").ServeHTTP))
		p.httpd.Handle(p.cfg.Prefix+"/debug/pprof/goroutine", p.midSecure(pprof.Handler("goroutine").ServeHTTP))
		p.httpd.Handle(p.cfg.Prefix+"/debug/pprof/heap", p.midSecure(pprof.Handler("heap").ServeHTTP))
		p.httpd.Handle(p.cfg.Prefix+"/debug/pprof/mutex", p.midSecure(pprof.Handler("mutex").ServeHTTP))
		p.httpd.Handle(p.cfg.Prefix+"/debug/pprof/threadcreate", p.midSecure(pprof.Handler("threadcreate").ServeHTTP))
	}

	if p.cfg.EnableWebPrettyTrace {
		p.httpd.Handle(p.cfg.Prefix+"/debug/prettytrace", p.midSecure(p.prettytrace))
	}

	if p.cfg.EnableGoPs {
		err := agent.Listen(agent.Options{Addr: p.cfg.GoPsAddr})
		if err != nil {
			if !p.cfg.SkipStartupErr {
				return errs.Wrap(err, "init gops agent failed")
			}
			p.Error("init gops agent failed", err, "addr", p.cfg.GoPsAddr)
		}
	}

	return p.Runner.Init()
}

func (p *profd) Stop() error {
	if p.cfg.EnableStartupProfiling && prof.IsRunning() {
		err := prof.Stop()
		if err != nil {
			p.Warn("prof stop failed when stopping", "err", err)
		}
	}
	return nil
}

func (p *profd) SetCfg(cfg any) {
	p.cfg = cfg.(*Cfg)
}

func (p *profd) Cfg() Cfg {
	return *p.cfg
}

func (p *profd) SetAllowedIPsGetter(allowedIPsGetter func() map[string]struct{}) {
	p.allowedIPsGetter = allowedIPsGetter
}

func (p *profd) startProf(w http.ResponseWriter, r *http.Request) {
	paramDir := r.URL.Query().Get("dir")
	if paramDir == "" {
		paramDir = p.cfg.ProfOutputDir
	}
	paramTimeout := r.URL.Query().Get("timeout")
	timeout, _ := strconv.Atoi(paramTimeout)
	mode := r.PathValue("mode")
	filepath, done, err := prof.Start(mode, paramDir, timeout)
	if err != nil {
		httpu.RespBytes(w, http.StatusInternalServerError, []byte(err.Error()))
		return
	}
	p.Info("start profiling", "mode", mode, "dir", paramDir, "timeout", timeout, "filepath", filepath)
	if done != nil {
		go func() {
			select {
			case <-done:
				p.Info("profiling done", "mode", mode, "dir", paramDir, "timeout", timeout, "filepath", filepath)
			case <-p.Stopping():
			}
		}()
	}
	httpu.RespBytes(w, http.StatusOK, []byte(filepath))
}

func (p *profd) stopProf(w http.ResponseWriter, r *http.Request) {
	err := prof.Stop()
	if err != nil {
		httpu.RespBytes(w, http.StatusInternalServerError, []byte(err.Error()))
	} else {
		httpu.RespBytes(w, http.StatusOK, []byte("stopped"))
	}
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

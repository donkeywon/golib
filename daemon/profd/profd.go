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
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/httpu"
	"github.com/donkeywon/golib/util/prof"
	"github.com/google/gops/agent"
	"github.com/maruel/panicparse/v2/stack/webstack"
)

const DaemonTypeProfd boot.DaemonType = "profd"

var D Profd = New()

type Profd interface {
	boot.Daemon
	SetMux(*http.ServeMux)
	SetAllowedIPsGetter(func() []string)
}

type profd struct {
	runner.Runner
	*Cfg

	mux *http.ServeMux

	allowedIPsGetter func() []string

	prettystackMu       sync.Mutex
	prettystackLastTime time.Time

	statsvizServer *statsviz.Server
}

func New() Profd {
	return &profd{
		Runner: runner.Create(string(DaemonTypeProfd)),
	}
}

func (p *profd) Init() error {
	if p.mux == nil {
		p.mux = httpd.D.Mux()
	}

	if p.Cfg.EnableStartupProfiling {
		filepath, done, err := prof.Start(p.Cfg.StartupProfilingMode, p.Cfg.ProfOutputDir, p.Cfg.StartupProfilingSec)
		if err != nil {
			p.Error("startup profiling failed", err,
				"mode", p.Cfg.StartupProfilingMode,
				"duration", fmt.Sprintf("%ds", p.Cfg.StartupProfilingSec),
				"filepath", filepath)
		} else {
			p.Info("startup profiling",
				"mode", p.Cfg.StartupProfilingMode,
				"duration", fmt.Sprintf("%ds", p.Cfg.StartupProfilingSec),
				"filepath", filepath)
		}
		if done != nil {
			go func() {
				select {
				case <-done:
					p.Info("startup profiling done",
						"mode", p.Cfg.StartupProfilingMode,
						"duration", fmt.Sprintf("%ds", p.Cfg.StartupProfilingSec),
						"filepath", filepath)
				case <-p.Stopping():
					return
				}
			}()
		}
	}

	var err error
	if p.Cfg.EnableStatsViz {
		p.statsvizServer, err = statsviz.NewServer()
		if err != nil {
			p.Error("init statsviz failed", err)
		} else {
			p.mux.HandleFunc("/debug/statsviz/", p.statsviz)
			p.mux.HandleFunc("/debug/statsviz/ws", p.statsvizWS)
		}
	}

	if p.Cfg.EnableHTTPProf {
		p.mux.Handle("/debug/prof/start/{mode}", httpd.RawHandler(p.startProf))
		p.mux.Handle("/debug/prof/stop", httpd.RawHandler(p.stopProf))
	}

	if p.Cfg.EnableWebProf {
		p.mux.HandleFunc("/debug/pprof/", p.pprofIndex)
		p.mux.HandleFunc("/debug/pprof/cmdline", p.pprofCmdline)
		p.mux.HandleFunc("/debug/pprof/profile", p.pprofProfile)
		p.mux.HandleFunc("/debug/pprof/symbol", p.pprofSymbol)
		p.mux.HandleFunc("/debug/pprof/trace", p.pprofTrace)
	}

	if p.Cfg.EnableWebPrettyTrace {
		p.mux.HandleFunc("/debug/prettytrace", p.prettytrace)
	}

	if p.Cfg.EnableGoPs {
		err := agent.Listen(agent.Options{Addr: p.Cfg.GoPsAddr})
		if err != nil {
			p.Error("init gops agent failed", err, "addr", p.Cfg.GoPsAddr)
		}
	}

	return p.Runner.Init()
}

func (p *profd) Stop() error {
	if p.Cfg.EnableStartupProfiling && prof.IsRunning() {
		err := prof.Stop()
		if err != nil {
			p.Warn("prof stop fail when stopping", "err", err)
		}
	}
	return nil
}

func (p *profd) SetMux(mux *http.ServeMux) {
	p.mux = mux
}

func (p *profd) SetAllowedIPsGetter(allowedIPsGetter func() []string) {
	p.allowedIPsGetter = allowedIPsGetter
}

func (p *profd) startProf(w http.ResponseWriter, r *http.Request) []byte {
	paramDir := r.URL.Query().Get("dir")
	if paramDir == "" {
		paramDir = p.Cfg.ProfOutputDir
	}
	paramTimeout := r.URL.Query().Get("timeout")
	timeout, _ := strconv.Atoi(paramTimeout)
	mode := r.PathValue("mode")
	filepath, done, err := prof.Start(mode, paramDir, timeout)
	if err != nil {
		return []byte(err.Error())
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
	return []byte(filepath)
}

func (p *profd) stopProf(w http.ResponseWriter, r *http.Request) []byte {
	err := prof.Stop()
	if err != nil {
		return []byte(err.Error())
	}
	return []byte("stopped")
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

	m := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		m[ip] = struct{}{}
	}

	remoteIP := httpu.GetRealRemoteIP(r)
	if _, exist := m[remoteIP]; exist {
		return true
	}

	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	return false
}

func (p *profd) auth(w http.ResponseWriter, r *http.Request) bool {
	if p.Cfg.WebAuthUser == "" && p.Cfg.WebAuthPwd == "" {
		return true
	}
	user, pass, ok := r.BasicAuth()
	if ok && user == p.Cfg.WebAuthUser && pass == p.Cfg.WebAuthPwd {
		return true
	}

	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted", charset="UTF-8"`)
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	return false

}

func (p *profd) pprofIndex(w http.ResponseWriter, r *http.Request) {
	if !p.secure(w, r) {
		return
	}

	pprof.Index(w, r)
}

func (p *profd) pprofProfile(w http.ResponseWriter, r *http.Request) {
	if !p.secure(w, r) {
		return
	}

	pprof.Profile(w, r)
}

func (p *profd) pprofSymbol(w http.ResponseWriter, r *http.Request) {
	if !p.secure(w, r) {
		return
	}

	pprof.Symbol(w, r)
}

func (p *profd) pprofTrace(w http.ResponseWriter, r *http.Request) {
	if !p.secure(w, r) {
		return
	}

	pprof.Trace(w, r)
}

func (p *profd) pprofCmdline(w http.ResponseWriter, r *http.Request) {
	if !p.secure(w, r) {
		return
	}

	pprof.Cmdline(w, r)
}

func (p *profd) prettytrace(w http.ResponseWriter, r *http.Request) {
	if !p.secure(w, r) {
		return
	}

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

func (p *profd) statsviz(w http.ResponseWriter, r *http.Request) {
	if !p.secure(w, r) {
		return
	}

	p.statsvizServer.Index()(w, r)
}

func (p *profd) statsvizWS(w http.ResponseWriter, r *http.Request) {
	if !p.secure(w, r) {
		return
	}

	p.statsvizServer.Ws()(w, r)
}

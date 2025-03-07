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
	"github.com/donkeywon/golib/util/prof"
	"github.com/google/gops/agent"
	"github.com/maruel/panicparse/v2/stack/webstack"
)

const DaemonTypeProfd boot.DaemonType = "profd"

var D Profd = New()

type Profd interface {
	runner.Runner
}

type profd struct {
	runner.Runner
	*Cfg

	prettystackMu       sync.Mutex
	prettystackLastTime time.Time
}

func New() Profd {
	return &profd{
		Runner: runner.Create(string(DaemonTypeProfd)),
	}
}

func (p *profd) Init() error {
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

	if p.Cfg.EnableStatsViz {
		srv, err := statsviz.NewServer()
		if err != nil {
			p.Error("init statsviz failed", err)
		} else {
			httpd.D.Handle("/debug/statsviz/", srv.Index())
			httpd.D.HandleFunc("/debug/statsviz/ws", srv.Ws())
		}
	}

	if p.Cfg.EnableHTTPProf {
		httpd.D.Handle("/debug/prof/start/{mode}", httpd.RawHandler(p.startProf))
		httpd.D.Handle("/debug/prof/stop", httpd.RawHandler(p.stopProf))
	}

	if p.Cfg.EnableWebProf {
		httpd.D.HandleFunc("/debug/pprof/", pprof.Index)
		httpd.D.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		httpd.D.HandleFunc("/debug/pprof/profile", pprof.Profile)
		httpd.D.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		httpd.D.HandleFunc("/debug/pprof/trace", pprof.Trace)
		httpd.D.HandleFunc("/debug/prettytrace", p.prettystack)
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

func (p *profd) Type() any {
	return DaemonTypeProfd
}

func (p *profd) GetCfg() any {
	return p.Cfg
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
	p.Info("start profiling", "mode", mode, "dir", paramDir, "timeout", timeout, filepath, "filepath")
	if done != nil {
		go func() {
			select {
			case <-done:
				p.Info("profiling done", "mode", mode, "dir", paramDir, "timeout", timeout, filepath, "filepath")
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

func (p *profd) prettystack(w http.ResponseWriter, req *http.Request) {
	p.prettystackMu.Lock()
	defer p.prettystackMu.Unlock()

	// Throttle requests.
	if time.Since(p.prettystackLastTime) < time.Second {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	webstack.SnapshotHandler(w, req)
	p.prettystackLastTime = time.Now()
}

package profd

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"strconv"

	"github.com/arl/statsviz"
	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/daemon/httpd"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/prof"
	"github.com/google/gops/agent"
)

const DaemonTypeProfd boot.DaemonType = "profd"

var (
	_p        = New()
	D  *Profd = _p
)

type Profd struct {
	runner.Runner
	*Cfg
}

func New() *Profd {
	return &Profd{
		Runner: runner.Create(string(DaemonTypeProfd)),
	}
}

func (p *Profd) Init() error {
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
	}

	if p.Cfg.EnableGoPs {
		err := agent.Listen(agent.Options{Addr: p.Cfg.GoPsAddr})
		if err != nil {
			p.Error("init gops agent failed", err, "addr", p.Cfg.GoPsAddr)
		}
	}

	return p.Runner.Init()
}

func (p *Profd) Stop() error {
	if p.Cfg.EnableStartupProfiling && prof.IsRunning() {
		err := prof.Stop()
		if err != nil {
			p.Warn("prof stop fail when stopping", "err", err)
		}
	}
	return nil
}

func (p *Profd) Type() any {
	return DaemonTypeProfd
}

func (p *Profd) GetCfg() any {
	return p.Cfg
}

func (p *Profd) startProf(w http.ResponseWriter, r *http.Request) []byte {
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

func (p *Profd) stopProf(w http.ResponseWriter, r *http.Request) []byte {
	err := prof.Stop()
	if err != nil {
		return []byte(err.Error())
	}
	return []byte("stopped")
}

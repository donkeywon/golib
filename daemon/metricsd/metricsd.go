package metricsd

import (
	"context"
	"net/http"

	"github.com/VictoriaMetrics/metrics"
	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/daemon/httpd"
	"github.com/donkeywon/golib/runner"
	"github.com/rs/zerolog"
)

const DaemonTypeMetricsd boot.DaemonType = "metricsd"

var _ Metricsd = (*metricsd)(nil)

type Metricsd interface {
	boot.Daemon
	SetGauge(string, float64)
	AddGauge(string, float64)
	IncGauge(string)
	DecGauge(string)
	IncCounter(string)
	AddCounter(string, int)
	Metrics() *metrics.Set
}

type metricsd struct {
	runner.Base

	cfg Cfg

	httpd httpd.HTTPd
	l     *zerolog.Logger
}

func New() boot.Daemon {
	return &metricsd{}
}

func (p *metricsd) Init(ctx context.Context) error {
	p.l = zerolog.Ctx(ctx)

	p.httpd = boot.Get[httpd.HTTPd](httpd.DaemonTypeHTTPd)
	p.httpd.Handle(p.cfg.HTTPEndpointPath, http.HandlerFunc(p.httpHandler))

	return nil
}

func (p *metricsd) SetCfg(cfg any) {
	p.cfg = cfg.(Cfg)
}

func (p *metricsd) SetGauge(name string, v float64) {
	metrics.GetOrCreateGauge(name, nil).Set(v)
}

func (p *metricsd) AddGauge(name string, v float64) {
	metrics.GetOrCreateGauge(name, nil).Add(v)
}

func (p *metricsd) IncGauge(name string) {
	metrics.GetOrCreateGauge(name, nil).Inc()
}

func (p *metricsd) DecGauge(name string) {
	metrics.GetOrCreateGauge(name, nil).Dec()
}

func (p *metricsd) IncCounter(name string) {
	metrics.GetOrCreateCounter(name).Inc()
}

func (p *metricsd) AddCounter(name string, v int) {
	metrics.GetOrCreateCounter(name).Add(v)
}

func (p *metricsd) AddCountInt64(name string, v int64) {
	metrics.GetOrCreateCounter(name).AddInt64(v)
}

func (p *metricsd) Metrics() *metrics.Set {
	return metrics.GetDefaultSet()
}

func (p *metricsd) httpHandler(w http.ResponseWriter, r *http.Request) {
	metrics.WritePrometheus(w, p.cfg.EnableProcCollector)
	if p.cfg.EnableFDCollector {
		metrics.WriteFDMetrics(w)
	}
}

package promd

import (
	"plugin"
	"reflect"
	"sync"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/svc/httpd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const SvcTypePromd boot.SvcType = "promd"

type Promd struct {
	runner.Runner
	plugin.Plugin
	*Cfg

	m   sync.Map
	reg *prometheus.Registry
}

var _p = &Promd{
	Runner: runner.NewBase(string(SvcTypePromd)),
	reg:    prometheus.NewRegistry(),
}

func New() *Promd {
	return _p
}

func (p *Promd) Init() error {
	if p.EnableGoCollector {
		p.reg.MustRegister(collectors.NewGoCollector())
	}
	if p.EnableProcCollector {
		p.reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	p.registerHTTPHandler()
	return p.Runner.Init()
}

func (p *Promd) Type() interface{} {
	return SvcTypePromd
}

func (p *Promd) GetCfg() interface{} {
	return p.Cfg
}

func (p *Promd) registerHTTPHandler() {
	httpd.Handle("/metrics", promhttp.HandlerFor(p.reg, promhttp.HandlerOpts{Registry: p.reg}))
}

func (p *Promd) SetGauge(name string, v float64) {
	p.opGauge(name, func(g prometheus.Gauge) { g.Set(v) })
}

func (p *Promd) AddGauge(name string, v float64) {
	p.opGauge(name, func(g prometheus.Gauge) { g.Add(v) })
}

func (p *Promd) SubGauge(name string, v float64) {
	p.opGauge(name, func(g prometheus.Gauge) { g.Sub(v) })
}

func (p *Promd) IncGauge(name string) {
	p.opGauge(name, func(g prometheus.Gauge) { g.Inc() })
}

func (p *Promd) DecGauge(name string) {
	p.opGauge(name, func(g prometheus.Gauge) { g.Dec() })
}

func (p *Promd) IncCounter(name string) {
	p.opCounter(name, func(c prometheus.Counter) { c.Inc() })
}

func (p *Promd) AddCounter(name string, v float64) {
	p.opCounter(name, func(c prometheus.Counter) { c.Add(v) })
}

func (p *Promd) loadOrStore(name string, m prometheus.Metric) prometheus.Metric {
	v, loaded := p.m.LoadOrStore(name, m)
	if !loaded {
		err := p.reg.Register(v.(prometheus.Collector))
		if err != nil {
			p.Error("register metrics fail", err, "name", name)
		}
	}
	return v.(prometheus.Metric)
}

func (p *Promd) opGauge(name string, op func(g prometheus.Gauge)) {
	g := p.loadOrStore(name, prometheus.NewGauge(prometheus.GaugeOpts{Name: name}))

	if gg, ok := g.(prometheus.Gauge); ok {
		op(gg)
		return
	}
	p.Warn("metrics type not match", "name", name, "wanted", "Gauge", "actual", reflect.TypeOf(g))
}

func (p *Promd) opCounter(name string, op func(c prometheus.Counter)) {
	c := p.loadOrStore(name, prometheus.NewCounter(prometheus.CounterOpts{Name: name}))

	if cc, ok := c.(prometheus.Counter); ok {
		op(cc)
		return
	}
	p.Warn("metrics type not match", "name", name, "wanted", "Counter", "actual", reflect.TypeOf(c))
}

func SetGauge(name string, v float64) {
	_p.SetGauge(name, v)
}

func AddGauge(name string, v float64) {
	_p.AddGauge(name, v)
}

func SubGauge(name string, v float64) {
	_p.SubGauge(name, v)
}

func IncGauge(name string) {
	_p.IncGauge(name)
}

func DecGauge(name string) {
	_p.DecGauge(name)
}

func IncCounter(name string) {
	_p.IncCounter(name)
}

func AddCounter(name string, v float64) {
	_p.AddCounter(name, v)
}

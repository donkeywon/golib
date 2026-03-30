package metricsd

import (
	"reflect"
	"sync"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/daemon/httpd"
	"github.com/donkeywon/golib/runner"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const DaemonTypeMetricsd boot.DaemonType = "metricsd"

var _ Metricsd = (*metricsd)(nil)

type Metricsd interface {
	boot.Daemon
	Register(prometheus.Collector) error
	MustRegister(...prometheus.Collector)
	RegisterMetric(prometheus.Metric) error
	SetGauge(string, float64)
	AddGauge(string, float64)
	SubGauge(string, float64)
	IncGauge(string)
	DecGauge(string)
	IncCounter(string)
	AddCounter(string, float64)
}

type metricsd struct {
	runner.Runner

	cfg   *Cfg
	mu    sync.RWMutex
	m     map[string]prometheus.Metric
	reg   *prometheus.Registry
	httpd httpd.HTTPd
}

func New() boot.Daemon {
	return &metricsd{
		Runner: runner.Create(string(DaemonTypeMetricsd)),
		reg:    prometheus.NewRegistry(),
		m:      make(map[string]prometheus.Metric),
	}
}

func (p *metricsd) Init() error {
	if !p.cfg.DisableGoCollector {
		p.reg.MustRegister(collectors.NewGoCollector())
	}
	if !p.cfg.DisableProcCollector {
		p.reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	p.httpd = boot.Get[httpd.HTTPd](boot.DaemonType("httpd"))
	p.httpd.Handle(p.cfg.HTTPEndpointPath, promhttp.HandlerFor(p.reg, promhttp.HandlerOpts{Registry: p.reg}))

	return p.Runner.Init()
}

func (p *metricsd) SetCfg(cfg any) {
	p.cfg = cfg.(*Cfg)
}

func (p *metricsd) SetGauge(name string, v float64) {
	p.opGauge(name, func(g prometheus.Gauge) { g.Set(v) })
}

func (p *metricsd) AddGauge(name string, v float64) {
	p.opGauge(name, func(g prometheus.Gauge) { g.Add(v) })
}

func (p *metricsd) SubGauge(name string, v float64) {
	p.opGauge(name, func(g prometheus.Gauge) { g.Sub(v) })
}

func (p *metricsd) IncGauge(name string) {
	p.opGauge(name, func(g prometheus.Gauge) { g.Inc() })
}

func (p *metricsd) DecGauge(name string) {
	p.opGauge(name, func(g prometheus.Gauge) { g.Dec() })
}

func (p *metricsd) IncCounter(name string) {
	p.opCounter(name, func(c prometheus.Counter) { c.Inc() })
}

func (p *metricsd) AddCounter(name string, v float64) {
	p.opCounter(name, func(c prometheus.Counter) { c.Add(v) })
}

func (p *metricsd) loadOrStore(name string, creator func() prometheus.Metric) prometheus.Metric {
	p.mu.RLock()
	m, exists := p.m[name]
	if exists {
		p.mu.RUnlock()
		return m
	}
	p.mu.RUnlock()

	p.mu.Lock()
	m, exists = p.m[name]
	if exists {
		p.mu.Unlock()
		return m
	}
	defer p.mu.Unlock()

	m = creator()
	err := p.reg.Register(m.(prometheus.Collector))
	if err != nil {
		p.Error("register metrics failed", err, "name", name)
		return m
	}

	p.m[name] = m
	return m
}

func (p *metricsd) opGauge(name string, op func(g prometheus.Gauge)) {
	g := p.loadOrStore(name, func() prometheus.Metric { return prometheus.NewGauge(prometheus.GaugeOpts{Name: name}) })

	if gg, ok := g.(prometheus.Gauge); ok {
		op(gg)
		return
	}
	p.Warn("metrics type not match", "name", name, "wanted", "Gauge", "actual", reflect.TypeOf(g))
}

func (p *metricsd) opCounter(name string, op func(c prometheus.Counter)) {
	c := p.loadOrStore(name, func() prometheus.Metric { return prometheus.NewCounter(prometheus.CounterOpts{Name: name}) })

	if cc, ok := c.(prometheus.Counter); ok {
		op(cc)
		return
	}
	p.Warn("metrics type not match", "name", name, "wanted", "Counter", "actual", reflect.TypeOf(c))
}

func (p *metricsd) Register(c prometheus.Collector) error {
	return p.reg.Register(c)
}

func (p *metricsd) MustRegister(c ...prometheus.Collector) {
	p.reg.MustRegister(c...)
}

func (p *metricsd) RegisterMetric(m prometheus.Metric) error {
	return p.Register(m.(prometheus.Collector))
}

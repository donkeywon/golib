package ds

import (
	"database/sql"
	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/daemon/promd"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/prometheus/client_golang/prometheus"
)

const DaemonTypeDS boot.DaemonType = "ds"

var (
	_d = New()

	fqNamespace    = string(DaemonTypeDS)
	fqSubsystem    = "pool_stats"
	variableLabels = []string{"name", "type"}

	maxOpenConnectionsDesc = prometheus.NewDesc(prometheus.BuildFQName(fqNamespace, fqSubsystem, "max_open_connections"), "Maximum number of open connections to the database.", variableLabels, nil)
	openConnectionsDesc    = prometheus.NewDesc(prometheus.BuildFQName(fqNamespace, fqSubsystem, "open_connections"), "The number of established connections both in use and idle.", variableLabels, nil)
	inUseConnectionsDesc   = prometheus.NewDesc(prometheus.BuildFQName(fqNamespace, fqSubsystem, "in_use"), "The number of connections currently in use.", variableLabels, nil)
	idleConnectionsDesc    = prometheus.NewDesc(prometheus.BuildFQName(fqNamespace, fqSubsystem, "idle"), "The number of idle connections.", variableLabels, nil)

	waitCountDesc         = prometheus.NewDesc(prometheus.BuildFQName(fqNamespace, fqSubsystem, "wait_count"), "The total number of connections waited for.", variableLabels, nil)
	waitDurationDesc      = prometheus.NewDesc(prometheus.BuildFQName(fqNamespace, fqSubsystem, "wait_duration"), "The total time blocked waiting for a new connection.", variableLabels, nil)
	maxIdleClosedDesc     = prometheus.NewDesc(prometheus.BuildFQName(fqNamespace, fqSubsystem, "max_idle_closed"), "The total number of connections closed due to SetMaxIdleConns.", variableLabels, nil)
	maxIdleTimeClosedDesc = prometheus.NewDesc(prometheus.BuildFQName(fqNamespace, fqSubsystem, "max_idle_time_closed"), "The total number of connections closed due to SetConnMaxIdleTime.", variableLabels, nil)
	maxLifetimeClosedDesc = prometheus.NewDesc(prometheus.BuildFQName(fqNamespace, fqSubsystem, "max_life_time_closed"), "The total number of connections closed due to SetConnMaxLifeTime.", variableLabels, nil)
)

type DS struct {
	runner.Runner
	plugin.Plugin
	*Cfg

	dbs map[string]*sql.DB
}

func D() *DS {
	return _d
}

func New() *DS {
	return &DS{
		Runner: runner.Create(string(DaemonTypeDS)),
		dbs:    make(map[string]*sql.DB),
	}
}

func (d *DS) Init() error {
	for _, ds := range d.Cfg.DS {
		db, err := sql.Open(ds.Type, ds.DSN)
		if err != nil {
			return errs.Wrapf(err, "open datasource fail, name: %s, type: %s, dsn: %s", ds.Name, ds.Type, ds.DSN)
		}

		db.SetMaxIdleConns(ds.MaxIdle)
		db.SetMaxOpenConns(ds.MaxOpen)
		db.SetConnMaxLifetime(ds.MaxLifeTime)
		db.SetConnMaxIdleTime(ds.MaxIdleTime)
		d.dbs[ds.Name] = db
	}
	if d.Cfg.EnableExportMetrics {
		promd.D().MustRegister(d)
	}
	return d.Runner.Init()
}

func (d *DS) Stop() error {
	for _, ds := range d.Cfg.DS {
		db := d.dbs[ds.Name]
		err := db.Close()
		if err != nil {
			d.Error("close db fail", err, "name", ds.Name, "type", ds.Type)
		}
	}
	return nil
}

func (d *DS) GetCfg() interface{} {
	return d.Cfg
}

func (d *DS) Type() interface{} {
	return DaemonTypeDS
}

func (d *DS) Describe(ch chan<- *prometheus.Desc) {
	ch <- maxOpenConnectionsDesc
	ch <- openConnectionsDesc
	ch <- inUseConnectionsDesc
	ch <- idleConnectionsDesc
	ch <- waitCountDesc
	ch <- waitDurationDesc
	ch <- maxIdleClosedDesc
	ch <- maxIdleTimeClosedDesc
	ch <- maxLifetimeClosedDesc
}

func (d *DS) Collect(ch chan<- prometheus.Metric) {
	for _, ds := range d.Cfg.DS {
		db := d.dbs[ds.Name]
		stats := db.Stats()

		ch <- prometheus.MustNewConstMetric(maxOpenConnectionsDesc, prometheus.GaugeValue, float64(stats.MaxOpenConnections), ds.Name, ds.Type)
		ch <- prometheus.MustNewConstMetric(openConnectionsDesc, prometheus.GaugeValue, float64(stats.OpenConnections), ds.Name, ds.Type)
		ch <- prometheus.MustNewConstMetric(idleConnectionsDesc, prometheus.GaugeValue, float64(stats.Idle), ds.Name, ds.Type)
		ch <- prometheus.MustNewConstMetric(inUseConnectionsDesc, prometheus.GaugeValue, float64(stats.InUse), ds.Name, ds.Type)

		ch <- prometheus.MustNewConstMetric(waitCountDesc, prometheus.CounterValue, float64(stats.WaitCount), ds.Name, ds.Type)
		ch <- prometheus.MustNewConstMetric(waitDurationDesc, prometheus.CounterValue, float64(stats.WaitDuration), ds.Name, ds.Type)
		ch <- prometheus.MustNewConstMetric(maxIdleClosedDesc, prometheus.CounterValue, float64(stats.MaxIdleClosed), ds.Name, ds.Type)
		ch <- prometheus.MustNewConstMetric(maxIdleTimeClosedDesc, prometheus.CounterValue, float64(stats.MaxIdleTimeClosed), ds.Name, ds.Type)
		ch <- prometheus.MustNewConstMetric(maxLifetimeClosedDesc, prometheus.CounterValue, float64(stats.MaxLifetimeClosed), ds.Name, ds.Type)
	}
}

func (d *DS) Get(name string) *sql.DB {
	return d.dbs[name]
}

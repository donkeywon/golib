package db

import (
	"database/sql"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/daemon/metricsd"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/runner"
	"github.com/prometheus/client_golang/prometheus"
)

const DaemonTypeDB boot.DaemonType = "db"

var (
	D DB = New()

	fqNamespace    = string(DaemonTypeDB)
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

type DB interface {
	boot.Daemon
	Get(string) *sql.DB
}

type db struct {
	runner.Runner
	*Cfg

	dbs map[string]*sql.DB
}

func New() DB {
	return &db{
		Runner: runner.Create(string(DaemonTypeDB)),
		dbs:    make(map[string]*sql.DB),
	}
}

func (d *db) Init() error {
	for _, dbCfg := range d.Cfg.Pools {
		db, err := sql.Open(dbCfg.Type, dbCfg.DSN)
		if err != nil {
			return errs.Wrapf(err, "open datasource fail, name: %s, type: %s, dsn: %s", dbCfg.Name, dbCfg.Type, dbCfg.DSN)
		}

		db.SetMaxIdleConns(dbCfg.MaxIdle)
		db.SetMaxOpenConns(dbCfg.MaxOpen)
		db.SetConnMaxLifetime(dbCfg.MaxLifeTime)
		db.SetConnMaxIdleTime(dbCfg.MaxIdleTime)
		d.dbs[dbCfg.Name] = db
	}
	if d.Cfg.EnableExportMetrics {
		metricsd.D.MustRegister(d)
	}
	return d.Runner.Init()
}

func (d *db) Stop() error {
	for _, dbCfg := range d.Cfg.Pools {
		db := d.dbs[dbCfg.Name]
		err := db.Close()
		if err != nil {
			d.Error("close db failed", err, "name", dbCfg.Name, "type", dbCfg.Type)
		}
	}
	return nil
}

func (d *db) Describe(ch chan<- *prometheus.Desc) {
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

func (d *db) Collect(ch chan<- prometheus.Metric) {
	for _, dbCfg := range d.Cfg.Pools {
		db := d.dbs[dbCfg.Name]
		stats := db.Stats()

		ch <- prometheus.MustNewConstMetric(maxOpenConnectionsDesc, prometheus.GaugeValue, float64(stats.MaxOpenConnections), dbCfg.Name, dbCfg.Type)
		ch <- prometheus.MustNewConstMetric(openConnectionsDesc, prometheus.GaugeValue, float64(stats.OpenConnections), dbCfg.Name, dbCfg.Type)
		ch <- prometheus.MustNewConstMetric(idleConnectionsDesc, prometheus.GaugeValue, float64(stats.Idle), dbCfg.Name, dbCfg.Type)
		ch <- prometheus.MustNewConstMetric(inUseConnectionsDesc, prometheus.GaugeValue, float64(stats.InUse), dbCfg.Name, dbCfg.Type)

		ch <- prometheus.MustNewConstMetric(waitCountDesc, prometheus.CounterValue, float64(stats.WaitCount), dbCfg.Name, dbCfg.Type)
		ch <- prometheus.MustNewConstMetric(waitDurationDesc, prometheus.CounterValue, float64(stats.WaitDuration), dbCfg.Name, dbCfg.Type)
		ch <- prometheus.MustNewConstMetric(maxIdleClosedDesc, prometheus.CounterValue, float64(stats.MaxIdleClosed), dbCfg.Name, dbCfg.Type)
		ch <- prometheus.MustNewConstMetric(maxIdleTimeClosedDesc, prometheus.CounterValue, float64(stats.MaxIdleTimeClosed), dbCfg.Name, dbCfg.Type)
		ch <- prometheus.MustNewConstMetric(maxLifetimeClosedDesc, prometheus.CounterValue, float64(stats.MaxLifetimeClosed), dbCfg.Name, dbCfg.Type)
	}
}

func (d *db) Get(name string) *sql.DB {
	return d.dbs[name]
}

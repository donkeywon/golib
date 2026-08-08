package dbp

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"strconv"
	"time"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/daemon/metricsd"
	"github.com/donkeywon/golib/errs"
	"github.com/rs/zerolog"
)

const DaemonTypeDBP boot.DaemonType = "dbp"

var _ DBP = (*dbp)(nil)

type DBP interface {
	boot.Daemon
	Get(string) *sql.DB
}

type dbp struct {
	cfg      Cfg
	dbs      map[string]*sql.DB
	metricsd metricsd.Metricsd
	l        *zerolog.Logger
}

func New() boot.Daemon {
	return &dbp{
		dbs: make(map[string]*sql.DB),
	}
}

func (d *dbp) Init(ctx context.Context) error {
	d.l = zerolog.Ctx(ctx)
	for _, dbCfg := range d.cfg.Pools {
		db, err := sql.Open(dbCfg.Type, dbCfg.DSN)
		if err != nil {
			return errs.Wrapf(err, "open db failed, name: %s, type: %s", dbCfg.Name, dbCfg.Type)
		}

		db.SetMaxIdleConns(dbCfg.MaxIdle)
		db.SetMaxOpenConns(dbCfg.MaxOpen)
		db.SetConnMaxLifetime(dbCfg.MaxLifeTime)
		db.SetConnMaxIdleTime(dbCfg.MaxIdleTime)

		err = d.waitDBReady(ctx, db, dbCfg.Name, dbCfg.Type, dbCfg.MaxWaitReadyTime, dbCfg.ReadyQuery)
		if err != nil {
			d.closeAll()
			return errs.Wrapf(err, "wait db ready timed out, name: %s, type: %s", dbCfg.Name, dbCfg.Type)
		}

		d.dbs[dbCfg.Name] = db
	}
	if d.cfg.EnableExportMetrics {
		d.metricsd = boot.Get[metricsd.Metricsd](metricsd.DaemonTypeMetricsd)
		d.metricsd.Metrics().RegisterMetricsWriter(d.writeMetrics)
	}
	return nil
}

func (d *dbp) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (d *dbp) SetCfg(cfg any) {
	d.cfg = cfg.(Cfg)
}

func (d *dbp) waitDBReady(ctx context.Context, db *sql.DB, name string, typ string, maxWait time.Duration, readyQuery string) error {
	if maxWait == 0 {
		return d.checkDBReady(ctx, db, readyQuery)
	}

	ctx, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()

	var err error
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return errs.Wrap(err, "check db ready failed")
		case <-t.C:
			err = d.checkDBReady(ctx, db, readyQuery)
			if err == nil {
				return nil
			}

			t.Reset(time.Second)
			d.l.Warn().Err(err).Str("name", name).Str("type", typ).Msg("check db ready failed")
		}
	}
}

func (d *dbp) checkDBReady(ctx context.Context, db *sql.DB, query string) error {
	if query == "" {
		return db.PingContext(ctx)
	}

	_, err := db.ExecContext(ctx, query)
	return err
}

func (d *dbp) Stop(ctx context.Context) error {
	d.closeAll()
	return nil
}

func (d *dbp) closeAll() {
	for _, dbCfg := range d.cfg.Pools {
		db := d.dbs[dbCfg.Name]
		if db == nil {
			continue
		}
		err := db.Close()
		if err != nil {
			d.l.Error().Err(err).Str("name", dbCfg.Name).Str("type", dbCfg.Type).Msg("close db failed")
		}
	}
}

func (d *dbp) Get(name string) *sql.DB {
	return d.dbs[name]
}

func (d *dbp) writeMetrics(w io.Writer) {
	buf := bytes.NewBuffer(make([]byte, 0, 128))
	for i := range d.cfg.Pools {
		poolCfg := &d.cfg.Pools[i]
		db := d.dbs[poolCfg.Name]
		stats := db.Stats()

		writeMetric(w, buf, "db_pool_stats_max_open_connections", poolCfg, int64(stats.MaxOpenConnections))
		writeMetric(w, buf, "db_pool_stats_open_connections", poolCfg, int64(stats.OpenConnections))
		writeMetric(w, buf, "db_pool_stats_idle", poolCfg, int64(stats.Idle))
		writeMetric(w, buf, "db_pool_stats_in_use", poolCfg, int64(stats.InUse))

		writeMetric(w, buf, "db_pool_stats_wait_count", poolCfg, stats.WaitCount)
		writeMetric(w, buf, "db_pool_stats_wait_duration", poolCfg, int64(stats.WaitDuration))
		writeMetric(w, buf, "db_pool_stats_max_idle_closed", poolCfg, stats.MaxIdleClosed)
		writeMetric(w, buf, "db_pool_stats_max_idle_time_closed", poolCfg, stats.MaxIdleTimeClosed)
		writeMetric(w, buf, "db_pool_stats_max_life_time_closed", poolCfg, stats.MaxLifetimeClosed)
	}
}

func writeMetric(w io.Writer, buf *bytes.Buffer, metricsName string, poolCfg *PoolCfg, v int64) {
	buf.Reset()
	buf.WriteString(metricsName)
	writeLabels(buf, poolCfg)
	buf.Write(strconv.AppendInt(buf.AvailableBuffer(), v, 10))
	buf.WriteByte('\n')
	w.Write(buf.Bytes())
}

func writeLabels(buf *bytes.Buffer, poolCfg *PoolCfg) {
	buf.WriteString(`{name="`)
	buf.WriteString(poolCfg.Name)
	buf.WriteString(`",type="`)
	buf.WriteString(poolCfg.Type)
	buf.WriteString(`"} `)
}

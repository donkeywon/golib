package db

import (
	"time"
)

const (
	DefaultEnableExportMetrics = true
)

type DBCfg struct {
	Name        string        `yaml:"name"`
	Type        string        `yaml:"type"`
	DSN         string        `yaml:"dsn"`
	MaxIdle     int           `yaml:"maxIdle"`
	MaxOpen     int           `yaml:"maxOpen"`
	MaxLifeTime time.Duration `yaml:"maxLifeTime"`
	MaxIdleTime time.Duration `yaml:"maxIdleTime"`
}

type Cfg struct {
	DB                  []*DBCfg `yaml:"db"`
	EnableExportMetrics bool     `env:"DB_ENABLE_EXPORT_METRICS"   yaml:"enableExportMetrics"   flag-long:"db-enable-export-metrics"   flag-description:"export database conn pool metrics with prometheus protocol"`
}

func NewCfg() *Cfg {
	return &Cfg{
		EnableExportMetrics: DefaultEnableExportMetrics,
	}
}

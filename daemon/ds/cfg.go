package ds

import (
	"time"
)

const (
	DefaultEnableExportMetrics = true
)

type DSCfg struct {
	Name        string        `yaml:"name"`
	Type        string        `yaml:"type"`
	DSN         string        `yaml:"dsn"`
	MaxIdle     int           `yaml:"maxIdle"`
	MaxOpen     int           `yaml:"maxOpen"`
	MaxLifeTime time.Duration `yaml:"maxLifeTime"`
	MaxIdleTime time.Duration `yaml:"maxIdleTime"`
}

type Cfg struct {
	DS                  []*DSCfg `yaml:"ds"`
	EnableExportMetrics bool     `env:"DS_ENABLE_EXPORT_METRICS"   yaml:"enableExportMetrics"   flag-long:"ds-enable-export-metrics"   flag-description:"export datasource pool metrics with prometheus protocol"`
}

func NewCfg() *Cfg {
	return &Cfg{
		EnableExportMetrics: DefaultEnableExportMetrics,
	}
}

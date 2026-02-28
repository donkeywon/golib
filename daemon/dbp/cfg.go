package dbp

import (
	"time"
)

const (
	DefaultEnableExportMetrics = true
)

type PoolCfg struct {
	Name             string        `yaml:"name"`
	Type             string        `yaml:"type"`
	DSN              string        `yaml:"dsn"`
	MaxIdle          int           `yaml:"maxIdle"`
	MaxOpen          int           `yaml:"maxOpen"`
	MaxLifeTime      time.Duration `yaml:"maxLifeTime"`
	MaxIdleTime      time.Duration `yaml:"maxIdleTime"`
	MaxWaitReadyTime time.Duration `yaml:"maxWaitReadyTime"`
	ReadyQuery       string        `yaml:"readyQuery"`
}

type Cfg struct {
	Pools               []*PoolCfg `yaml:"pools"`
	EnableExportMetrics bool       `env:"ENABLE_EXPORT_METRICS" yaml:"enableExportMetrics" long:"enable-export-metrics" description:"export database conn pool metrics with prometheus protocol"`
}

func NewCfg() *Cfg {
	return &Cfg{
		EnableExportMetrics: DefaultEnableExportMetrics,
	}
}

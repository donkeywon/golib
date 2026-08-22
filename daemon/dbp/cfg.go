package dbp

import (
	"encoding/json/v2"
	"time"
)

const (
	DefaultEnableExportMetrics = true
)

type PoolCfg struct {
	Name             string        `json:"name"                yaml:"name"`
	Type             string        `json:"type"                yaml:"type"`
	DSN              string        `json:"dsn"                 yaml:"dsn"`
	MaxIdle          int           `json:"max_idle"            yaml:"maxIdle"`
	MaxOpen          int           `json:"max_open"            yaml:"maxOpen"`
	MaxLifeTime      time.Duration `json:"max_life_time"       yaml:"maxLifeTime"`
	MaxIdleTime      time.Duration `json:"max_idle_time"       yaml:"maxIdleTime"`
	MaxWaitReadyTime time.Duration `json:"max_wait_ready_time" yaml:"maxWaitReadyTime"`
	ReadyQuery       string        `json:"ready_query"         yaml:"readyQuery"`
}

func (pc *PoolCfg) UnmarshalFlag(value string) error {
	return json.Unmarshal([]byte(value), pc)
}

func (pc *PoolCfg) MarshalFlag() (string, error) {
	bs, err := json.Marshal(pc)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

type Cfg struct {
	Pools               []PoolCfg `env:"POOLS" env-delim:";"   yaml:"pools"               long:"pools"                 description:"database connection pools"                                 validate:"required"`
	EnableExportMetrics bool      `env:"ENABLE_EXPORT_METRICS" yaml:"enableExportMetrics" long:"enable-export-metrics" description:"export database conn pool metrics with prometheus protocol"`
}

func NewCfg() Cfg {
	return Cfg{
		EnableExportMetrics: DefaultEnableExportMetrics,
	}
}

package httpd

import (
	"time"
)

const (
	DefaultWriteTimeout      = 30 * time.Second
	DefaultReadTimeout       = 60 * time.Second
	DefaultReadHeaderTimeout = 5 * time.Second
	DefaultIdleTimeout       = 30 * time.Second
)

type Cfg struct {
	Addr              string        `env:"ADDR"                long:"addr"                yaml:"addr"      validate:"required" description:"http listen address"`
	WriteTimeout      time.Duration `env:"WRITE_TIMEOUT"       long:"write-timeout"       yaml:"writeTimeout"                  description:"maximum duration before timing out writes of the response"`
	ReadTimeout       time.Duration `env:"READ_TIMEOUT"        long:"read-timeout"        yaml:"readTimeout"                   description:"maximum duration for reading the entire request, including the body. A zero or negative value means there will be no timeout."`
	ReadHeaderTimeout time.Duration `env:"READ_HEADER_TIMEOUT" long:"read-header-timeout" yaml:"readHeaderTimeout"             description:"the amount of time allowed to read request headers"`
	IdleTimeout       time.Duration `env:"IDLE_TIMEOUT"        long:"idle-timeout"        yaml:"idleTimeout"                   description:"maximum amount of time to wait for the next request when keep-alives are enabled"`
}

func NewCfg() *Cfg {
	return &Cfg{
		WriteTimeout:      DefaultWriteTimeout,
		ReadTimeout:       DefaultReadTimeout,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		IdleTimeout:       DefaultIdleTimeout,
	}
}

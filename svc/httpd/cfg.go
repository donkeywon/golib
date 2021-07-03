package httpd

import (
	"time"
)

const (
	DefaultWriteTimeout      = time.Second
	DefaultReadTimeout       = time.Second
	DefaultReadHeaderTimeout = time.Second
	DefaultIdleTimeout       = time.Second
)

type Cfg struct {
	Addr              string        `env:"HTTPD_ADDR"                yaml:"addr"`
	WriteTimeout      time.Duration `env:"HTTPD_WRITE_TIMEOUT"       yaml:"writeTimeout"`
	ReadTimeout       time.Duration `env:"HTTPD_READ_TIMEOUT"        yaml:"readTimeout"`
	ReadHeaderTimeout time.Duration `env:"HTTPD_READ_HEADER_TIMEOUT" yaml:"readHeaderTimeout"`
	IdleTimeout       time.Duration `env:"HTTPD_IDLE_TIMEOUT"        yaml:"idleTimeout"`
}

func NewCfg() *Cfg {
	return &Cfg{
		WriteTimeout:      DefaultWriteTimeout,
		ReadTimeout:       DefaultReadTimeout,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		IdleTimeout:       DefaultIdleTimeout,
	}
}

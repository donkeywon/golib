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
	Addr              string        `env:"HTTPD_ADDR"                flag-long:"httpd-addr"                yaml:"addr"      validate:"required" flag-description:"http listen address"`
	WriteTimeout      time.Duration `env:"HTTPD_WRITE_TIMEOUT"       flag-long:"httpd-write-timeout"       yaml:"writeTimeout"                  flag-description:"maximum duration before timing out writes of the response"`
	ReadTimeout       time.Duration `env:"HTTPD_READ_TIMEOUT"        flag-long:"httpd-read-timeout"        yaml:"readTimeout"                   flag-description:"maximum duration for reading the entire request, including the body. A zero or negative value means there will be no timeout."`
	ReadHeaderTimeout time.Duration `env:"HTTPD_READ_HEADER_TIMEOUT" flag-long:"httpd-read-header-timeout" yaml:"readHeaderTimeout"             flag-description:"the amount of time allowed to read request headers"`
	IdleTimeout       time.Duration `env:"HTTPD_IDLE_TIMEOUT"        flag-long:"httpd-idle-timeout"        yaml:"idleTimeout"                   flag-description:"maximum amount of time to wait for the next request when keep-alives are enabled"`
}

func NewCfg() *Cfg {
	return &Cfg{
		WriteTimeout:      DefaultWriteTimeout,
		ReadTimeout:       DefaultReadTimeout,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		IdleTimeout:       DefaultIdleTimeout,
	}
}

package boot

import (
	"context"
	"time"

	"github.com/donkeywon/golib/logs"
	"github.com/rs/zerolog"
)

type OnCreatedFunc func(context.Context)

type Option func(*options)

type LoggerCreator interface {
	Create() (zerolog.Logger, error)
}

type options struct {
	CfgPath      string `env:"CFG_PATH" description:"config file path"   long:"config"  short:"c"`
	PrintVersion bool   `               description:"print version info" long:"version" short:"v"`

	loggerCfgKey      string
	loggerCreator     LoggerCreator
	envPrefix         string
	onCreated         map[DaemonType]OnCreatedFunc
	daemonStopTimeout time.Duration
}

func createOptions(opt ...Option) options {
	options := options{
		onCreated:         make(map[DaemonType]OnCreatedFunc),
		loggerCfgKey:      "log",
		loggerCreator:     logs.NewRotateLoggerCreator(),
		daemonStopTimeout: time.Second * 10,
	}

	for _, o := range opt {
		o(&options)
	}

	return options
}

func CfgPath(cfgPath string) Option {
	return func(o *options) {
		o.CfgPath = cfgPath
	}
}

func EnvPrefix(envPrefix string) Option {
	return func(o *options) {
		o.envPrefix = envPrefix
	}
}

func WithLoggerCreator(cfgKey string, c LoggerCreator) Option {
	if cfgKey == "" {
		panic("empty logger cfg key")
	}
	if c == nil {
		panic("nil logger creator")
	}
	return func(o *options) {
		o.loggerCfgKey = cfgKey
		o.loggerCreator = c
	}
}

func OnCreated(t DaemonType, f OnCreatedFunc) Option {
	return func(o *options) {
		o.onCreated[t] = f
	}
}

func DaemonStopTimeout(t time.Duration) Option {
	return func(o *options) {
		if t > 0 {
			o.daemonStopTimeout = t
		}
	}
}

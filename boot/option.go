package boot

import (
	"context"

	"github.com/donkeywon/golib/logs"
)

type OnCreatedFunc func(context.Context)

type Option func(*options)

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

func WithLoggerCreator(cfgKey string, c logs.Creator) Option {
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

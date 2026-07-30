package boot

import (
	"context"

	"github.com/donkeywon/golib/logs"
)

type OnCreatedFunc func(context.Context)

type Option func(*booter)

func CfgPath(cfgPath string) Option {
	return func(b *booter) {
		b.options.CfgPath = cfgPath
	}
}

func EnvPrefix(envPrefix string) Option {
	return func(b *booter) {
		b.options.envPrefix = envPrefix
	}
}

func WithLoggerCreator(cfgKey string, c logs.Creator) Option {
	if cfgKey == "" {
		panic("empty logger cfg key")
	}
	if c == nil {
		panic("nil logger creator")
	}
	return func(b *booter) {
		b.options.loggerCfgKey = cfgKey
		b.options.loggerCreator = c
	}
}

func OnCreated(t DaemonType, f OnCreatedFunc) Option {
	return func(b *booter) {
		b.options.onCreated[t] = f
	}
}

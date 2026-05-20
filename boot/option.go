package boot

import (
	"github.com/donkeywon/golib/logs"
)

type OnConfigLoadedFunc func(any)
type OnCreatedFunc func()
type OnInitializedFunc func()

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

func OnConfigLoaded(t DaemonType, f OnConfigLoadedFunc) Option {
	return func(b *booter) {
		b.options.onConfigLoaded[t] = f
	}
}

func OnCreated(t DaemonType, f OnCreatedFunc) Option {
	return func(b *booter) {
		b.options.onCreated[t] = f
	}
}

func OnInitialized(t DaemonType, f OnInitializedFunc) Option {
	return func(b *booter) {
		b.options.onInitialized[t] = f
	}
}

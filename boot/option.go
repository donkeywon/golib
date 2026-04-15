package boot

import "github.com/donkeywon/golib/log"

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

func DefaultLogCfg(cfg *log.Cfg) Option {
	return func(b *booter) {
		b.logCfg = cfg
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

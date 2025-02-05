package boot

import "github.com/donkeywon/golib/log"

type OnConfigLoadedFunc func(cfg map[string]any)

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

func OnConfigLoaded(f OnConfigLoadedFunc) Option {
	return func(b *booter) {
		b.options.onConfigLoaded = f
	}
}

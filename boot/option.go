package boot

import "github.com/donkeywon/golib/log"

type Option func(*Booter)

func CfgPath(cfgPath string) Option {
	return func(b *Booter) {
		b.options.CfgPath = cfgPath
	}
}

func EnvPrefix(envPrefix string) Option {
	return func(b *Booter) {
		b.options.EnvPrefix = envPrefix
	}
}

func DefaultLogCfg(cfg *log.Cfg) Option {
	return func(b *Booter) {
		b.logCfg = cfg
	}
}

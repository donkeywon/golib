package boot

import (
	"log/slog"
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

func WithLogHandler(h slog.Handler) Option {
	return func(b *booter) {
		b.options.logHandler = h
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

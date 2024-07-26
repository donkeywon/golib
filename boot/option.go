package boot

type Option interface {
	apply(*Booter)
}

type optionFunc func(*Booter)

func (f optionFunc) apply(b *Booter) {
	f(b)
}

func CfgPath(cfgPath string) Option {
	return optionFunc(func(b *Booter) {
		b.options.CfgPath = cfgPath
	})
}

func EnvPrefix(envPrefix string) Option {
	return optionFunc(func(b *Booter) {
		b.options.EnvPrefix = envPrefix
	})
}

func DefaultLogPath(logPath string) Option {
	return optionFunc(func(b *Booter) {
		b.logCfg.Filepath = logPath
	})
}

func DefaultLogMaxFileSize(maxSize int) Option {
	return optionFunc(func(b *Booter) {
		b.logCfg.MaxFileSize = maxSize
	})
}

func DefaultLogMaxBackups(maxBackups int) Option {
	return optionFunc(func(b *Booter) {
		b.logCfg.MaxBackups = maxBackups
	})
}

func DefaultLogMaxAge(maxAge int) Option {
	return optionFunc(func(b *Booter) {
		b.logCfg.MaxAge = maxAge
	})
}

func DefaultLogDisableCompress(disableCompress bool) Option {
	return optionFunc(func(b *Booter) {
		b.logCfg.DisableCompress = disableCompress
	})
}

func DefaultLogEncoding(encoding string) Option {
	return optionFunc(func(b *Booter) {
		b.logCfg.Format = encoding
	})
}

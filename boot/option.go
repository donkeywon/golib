package boot

type Option interface {
	apply(*Booter)
}

type optionFunc func(*Booter)

func (f optionFunc) apply(b *Booter) {
	f(b)
}

func EnvPrefix(envPrefix string) Option {
	return optionFunc(func(b *Booter) {
		b.envPrefix = envPrefix
	})
}

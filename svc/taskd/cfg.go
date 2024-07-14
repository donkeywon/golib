package taskd

const (
	DefaultPoolSize  = 5
	DefaultQueueSize = 1024
)

type Cfg struct {
	PoolSize  int `env:"TASK_POOL_SIZE"  yaml:"poolSize"`
	QueueSize int `env:"TASK_QUEUE_SIZE" yaml:"queueSize"`
}

func NewCfg() *Cfg {
	return &Cfg{
		PoolSize:  DefaultPoolSize,
		QueueSize: DefaultQueueSize,
	}
}

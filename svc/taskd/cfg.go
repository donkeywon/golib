package taskd

const (
	DefaultPoolSize  = 5
	DefaultQueueSize = 1024
)

type Cfg struct {
	PoolSize  int `env:"TASK_POOL_SIZE"  flag-long:"task-pool-size"  yaml:"poolSize"  flag-description:"max number of workers in task pool"`
	QueueSize int `env:"TASK_QUEUE_SIZE" flag-long:"task-queue-size" yaml:"queueSize" flag-description:"max size of buffered task queue"`
}

func NewCfg() *Cfg {
	return &Cfg{
		PoolSize:  DefaultPoolSize,
		QueueSize: DefaultQueueSize,
	}
}

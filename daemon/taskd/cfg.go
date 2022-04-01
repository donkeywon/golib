package taskd

const (
	DefaultPool      = "default"
	DefaultPoolSize  = 64
	DefaultQueueSize = 1024
)

type PoolCfg struct {
	Name      string `yaml:"name" validate:"required"`
	Size      int    `yaml:"size" validate:"required"`
	QueueSize int    `yaml:"queueSize" validate:"required"`
}

type Cfg struct {
	Pools []*PoolCfg `yaml:"pools" env:"TASK_POOLS"`
}

func NewCfg() *Cfg {
	return &Cfg{
		Pools: []*PoolCfg{
			{
				Name:      DefaultPool,
				Size:      DefaultPoolSize,
				QueueSize: DefaultQueueSize,
			},
		},
	}
}

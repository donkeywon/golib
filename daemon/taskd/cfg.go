package taskd

const (
	DefaultPool      = "default"
	DefaultPoolSize  = 64
	DefaultQueueSize = 1024
)

type PoolCfg struct {
	Name      string `json:"name" yaml:"name" validate:"required"`
	Size      int    `json:"size" yaml:"size" validate:"required"`
	QueueSize int    `json:"queueSize" yaml:"queueSize" validate:"required"`
}

type Cfg struct {
	Pools []*PoolCfg `json:"pools" yaml:"pools" env:"POOLS"`
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

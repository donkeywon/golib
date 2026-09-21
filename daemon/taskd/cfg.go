package taskd

import "time"

const (
	DefaultPoolSize        = 64
	DefaultQueueSize       = 1024
	DefaultShutdownTimeout = time.Second * 30
)

type Cfg struct {
	Size            int           `json:"size"             yaml:"size"            validate:"gt=0"`
	QueueSize       int           `json:"queue_size"       yaml:"queueSize"       validate:"gt=0"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdownTimeout" validate:"gte=0"`
}

func NewCfg() Cfg {
	return Cfg{
		Size:            DefaultPoolSize,
		QueueSize:       DefaultQueueSize,
		ShutdownTimeout: DefaultShutdownTimeout,
	}
}

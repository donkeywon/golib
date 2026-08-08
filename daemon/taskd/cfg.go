package taskd

import "time"

const (
	DefaultPoolSize        = 64
	DefaultQueueSize       = 1024
	DefaultShutdownTimeout = time.Second * 30
)

type Cfg struct {
	Size            int           `json:"size"             yaml:"size"            validate:"required"`
	QueueSize       int           `json:"queue_size"       yaml:"queueSize"       validate:"required"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdownTimeout" validate:"required"`
}

func NewCfg() Cfg {
	return Cfg{
		Size:            DefaultPoolSize,
		QueueSize:       DefaultQueueSize,
		ShutdownTimeout: DefaultShutdownTimeout,
	}
}

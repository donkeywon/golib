package promd

const (
	DefaultDisableGoCollector   = false
	DefaultDisableProcCollector = false
)

type Cfg struct {
	DisableGoCollector   bool `env:"PROMETHEUS_DISABLE_GO_COLLECTOR"   flag-long:"prom-disable-go-collector"   yaml:"disableGoCollector" flag-description:"disable collect current go process runtime metrics"`
	DisableProcCollector bool `env:"PROMETHEUS_DISABLE_PROC_COLLECTOR" flag-long:"prom-disable-proc-collector" yaml:"disableProcCollector" flag-description:"disable collect current state of process metrics including CPU, memory and file descriptor usage as well as the process start time"`
}

func NewCfg() *Cfg {
	return &Cfg{
		DisableGoCollector:   DefaultDisableGoCollector,
		DisableProcCollector: DefaultDisableProcCollector,
	}
}

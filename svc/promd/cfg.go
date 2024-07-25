package promd

const (
	DefaultEnableGoCollector   = true
	DefaultEnableProcCollector = true
)

type Cfg struct {
	EnableGoCollector   bool `env:"PROMETHEUS_ENABLE_GO_COLLECTOR"   flag-long:"prom-enable-go-collector"   yaml:"enableGoCollector" flag-description:"enable collect current go process runtime metrics"`
	EnableProcCollector bool `env:"PROMETHEUS_ENABLE_PROC_COLLECTOR" flag-long:"prom-enable-proc-collector" yaml:"enableProcCollector" flag-description:"enable collect current state of process metrics including CPU, memory and file descriptor usage as well as the process start time"`
}

func NewCfg() *Cfg {
	return &Cfg{
		EnableGoCollector:   DefaultEnableGoCollector,
		EnableProcCollector: DefaultEnableProcCollector,
	}
}

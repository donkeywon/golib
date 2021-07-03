package promd

const (
	DefaultEnableGoCollector   = true
	DefaultEnableProcCollector = true
)

type Cfg struct {
	EnableGoCollector   bool `env:"PROMETHEUS_ENABLE_GO_COLLECTOR"   yaml:"enableGoCollector"`
	EnableProcCollector bool `env:"PROMETHEUS_ENABLE_PROC_COLLECTOR" yaml:"enableProcCollector"`
}

func NewCfg() *Cfg {
	return &Cfg{
		EnableGoCollector:   DefaultEnableGoCollector,
		EnableProcCollector: DefaultEnableProcCollector,
	}
}

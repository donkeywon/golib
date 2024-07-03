package promd

const (
	DefaultEnableGoCollector   = true
	DefaultEnableProcCollector = true
)

type Cfg struct {
	EnableGoCollector   bool `env:"PROMETHEUS_ENABLE_GO_COLLECTOR"   json:"enableGoCollector"   yaml:"enableGoCollector"`
	EnableProcCollector bool `env:"PROMETHEUS_ENABLE_PROC_COLLECTOR" json:"enableProcCollector" yaml:"enableProcCollector"`
}

func NewCfg() *Cfg {
	return &Cfg{
		EnableGoCollector:   DefaultEnableGoCollector,
		EnableProcCollector: DefaultEnableProcCollector,
	}
}

package promd

const (
	DefaultDisableGoCollector   = false
	DefaultDisableProcCollector = false
	DefaultHTTPEndpointPath     = "/metrics"
)

type Cfg struct {
	DisableGoCollector   bool   `env:"PROMETHEUS_DISABLE_GO_COLLECTOR"   yaml:"disableGoCollector"   flag-long:"prom-disable-go-collector"   flag-description:"disable collect current go process runtime metrics"`
	DisableProcCollector bool   `env:"PROMETHEUS_DISABLE_PROC_COLLECTOR" yaml:"disableProcCollector" flag-long:"prom-disable-proc-collector" flag-description:"disable collect current state of process metrics including CPU, memory and file descriptor usage as well as the process start time"`
	HTTPEndpointPath     string `env:"PROMETHEUS_HTTP_ENDPOINT_PATH"     yaml:"httpEndpointPath"     flag-long:"prom-http-endpoint-path"     flag-description:"metrics http endpoint path"`
}

func NewCfg() *Cfg {
	return &Cfg{
		DisableGoCollector:   DefaultDisableGoCollector,
		DisableProcCollector: DefaultDisableProcCollector,
		HTTPEndpointPath:     DefaultHTTPEndpointPath,
	}
}

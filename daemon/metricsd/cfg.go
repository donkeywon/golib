package metricsd

const (
	DefaultDisableGoCollector   = false
	DefaultDisableProcCollector = false
	DefaultHTTPEndpointPath     = "/metrics"
)

type Cfg struct {
	DisableGoCollector   bool   `env:"DISABLE_GO_COLLECTOR"   yaml:"disableGoCollector"   long:"disable-go-collector"   description:"disable collect current go process runtime metrics"`
	DisableProcCollector bool   `env:"DISABLE_PROC_COLLECTOR" yaml:"disableProcCollector" long:"disable-proc-collector" description:"disable collect current state of process metrics including CPU, memory and file descriptor usage as well as the process start time"`
	HTTPEndpointPath     string `env:"HTTP_ENDPOINT_PATH"     yaml:"httpEndpointPath"     long:"http-endpoint-path"     description:"metrics http endpoint path"`
}

func NewCfg() *Cfg {
	return &Cfg{
		DisableGoCollector:   DefaultDisableGoCollector,
		DisableProcCollector: DefaultDisableProcCollector,
		HTTPEndpointPath:     DefaultHTTPEndpointPath,
	}
}

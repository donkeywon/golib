package metricsd

const (
	DefaultEnableProcCollector = false
	DefaultEnableFDCollector   = false
	DefaultHTTPEndpointPath    = "/metrics"
)

type Cfg struct {
	EnableProcCollector bool   `env:"ENABLE_PROC_COLLECTOR" yaml:"enableProcCollector" long:"enable-proc-collector" description:"enable collect current state of process metrics including go runtime metrics, CPU, memory and file descriptor usage as well as the process start time"`
	EnableFDCollector   bool   `env:"ENABLE_FD_COLLECTOR"   yaml:"enableFDCollector"   long:"enable-fd-collector"   description:"enable file descriptor counter metrics(may affect performance)"`
	HTTPEndpointPath    string `env:"HTTP_ENDPOINT_PATH"    yaml:"httpEndpointPath"    long:"http-endpoint-path"    description:"metrics http endpoint path"`
}

func NewCfg() *Cfg {
	return &Cfg{
		EnableProcCollector: DefaultEnableProcCollector,
		EnableFDCollector:   DefaultEnableFDCollector,
		HTTPEndpointPath:    DefaultHTTPEndpointPath,
	}
}

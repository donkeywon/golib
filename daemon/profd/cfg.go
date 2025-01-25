package profd

const (
	DefaultEnableStartupProfiling = false
	DefaultStartupProfilingSec    = 300
	DefaultStartupProfilingMode   = "mem"
	DefaultProfilingOutputDir     = "/tmp"
	DefaultEnableGoPs             = false
	DefaultGoPsAddr               = ":"
	DefaultEnableHTTPProf         = false
	DefaultEnableWebProf          = false
	DefaultEnableWebPrettyTrace   = false
	DefaultEnableStatsViz         = false
)

type Cfg struct {
	EnableStartupProfiling bool   `yaml:"enableStartupProfiling"   env:"ENABLE_STARTUP_PROFILING"   long:"enable-startup-profiling" description:"profiling at startup"`
	StartupProfilingSec    int    `yaml:"startupProfilingSec"      env:"STARTUP_PROFILING_SEC"      long:"startup-profiling-sec"    description:"startup profiling duration in seconds, only works when prof-enable-startup-profiling is enabled"`
	StartupProfilingMode   string `yaml:"startupProfilingMode"     env:"STARTUP_PROFILING_MODE"     long:"startup-profiling-mode"   description:"startup profiling mode, only works when prof-enable-startup-profiling is enabled"`
	ProfOutputDir          string `yaml:"profOutputDir"            env:"OUTPUT_DIR"                 long:"output-dir"               description:"dir path of pprof file save to"`

	EnableHTTPProf       bool   `yaml:"enableHTTPProf" env:"ENABLE_HTTP_PROF" long:"enable-http-prof" description:"enable prof over http, depends on httpd"`
	EnableWebProf        bool   `yaml:"enableWebProf"  env:"ENABLE_WEB_PROF"  long:"enable-web-prof"  description:"enable prof over web, depends on httpd"`
	EnableWebPrettyTrace bool   `yaml:"enableWebPrettyTrace" env:"ENABLE_WEB_PRETTY_TRACE" long:"enable-web-pretty-trace" description:"enable pretty trace over web, depends on httpd"`
	WebAuthUser          string `yaml:"webAuthUser" env:"WEB_AUTH_USER"`
	WebAuthPwd           string `yaml:"webAuthPwd" env:"WEB_AUTH_PWD"`

	EnableGoPs bool   `yaml:"enableGoPs" env:"ENABLE_GOPS" long:"enable-gops" =description:"enable gops agent"`
	GoPsAddr   string `yaml:"goPsAddr"   env:"GOPS_ADDR"   long:"gops-addr"   =description:"gops agent listen addr"`

	EnableStatsViz bool `yaml:"enableStatsViz" env:"ENABLE_STATS_VIZ" long:"enable-stats-viz" description:"enable statsviz, need httpd"`
}

func NewCfg() *Cfg {
	return &Cfg{
		EnableStartupProfiling: DefaultEnableStartupProfiling,
		StartupProfilingSec:    DefaultStartupProfilingSec,
		StartupProfilingMode:   DefaultStartupProfilingMode,
		ProfOutputDir:          DefaultProfilingOutputDir,
		EnableGoPs:             DefaultEnableGoPs,
		GoPsAddr:               DefaultGoPsAddr,
		EnableHTTPProf:         DefaultEnableHTTPProf,
		EnableWebProf:          DefaultEnableWebProf,
		EnableWebPrettyTrace:   DefaultEnableWebPrettyTrace,
		EnableStatsViz:         DefaultEnableStatsViz,
	}
}

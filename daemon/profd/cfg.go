package profd

const (
	DefaultEnableStartupProfiling = false
	DefaultStartupProfilingSec    = 300
	DefaultStartupProfilingMode   = "mem"
	DefaultProfilingOutputDir     = "/tmp"
	DefaultEnableGoPs             = false
	DefaultGoPsAddr               = ":"
	DefaultEnableHTTPPprof        = false
	DefaultEnableStatsViz         = false
)

type Cfg struct {
	EnableStartupProfiling bool   `yaml:"enableStartupProfiling"   env:"PROF_ENABLE_STARTUP_PROFILING"   flag-long:"prof-enable-startup-profiling" flag-description:"profiling at startup"`
	StartupProfilingSec    int    `yaml:"startupProfilingSec"      env:"PROF_STARTUP_PROFILING_SEC"      flag-long:"prof-startup-profiling-sec"    flag-description:"startup profiling duration in seconds, only works when prof-enable-startup-profiling is enabled"`
	StartupProfilingMode   string `yaml:"startupProfilingMode"     env:"PROF_STARTUP_PROFILING_MODE"     flag-long:"prof-startup-profiling-mode"   flag-description:"startup profiling mode, only works when prof-enable-startup-profiling is enabled"`
	ProfOutputDir          string `yaml:"profOutputDir"            env:"PROF_OUTPUT_DIR"                 flag-long:"prof-output-dir"               flag-description:"dir path of pprof file save to"`

	EnableHTTPPprof bool `yaml:"enableHTTPPprof" env:"PROF_ENABLE_HTTP_PPROF" flag-long:"prof-enable-http-pprof" flag-description:"enable pprof over http, need httpd"`

	EnableGoPs bool   `yaml:"enableGoPs" env:"PROF_ENABLE_GOPS" flag-long:"prof-enable-gops" flag-description:"enable gops agent"`
	GoPsAddr   string `yaml:"goPsAddr"   env:"PROF_GOPS_ADDR"   flag-long:"prof-gops-addr"   flag-description:"gops agent listen addr"`

	EnableStatsViz bool `yaml:"enableStatsViz" env:"PROF_ENABLE_STATS_VIZ" flag-long:"prof-enable-stats-viz" flag-description:"enable statsviz, need httpd"`
}

func NewCfg() *Cfg {
	return &Cfg{
		EnableStartupProfiling: DefaultEnableStartupProfiling,
		StartupProfilingSec:    DefaultStartupProfilingSec,
		StartupProfilingMode:   DefaultStartupProfilingMode,
		ProfOutputDir:          DefaultProfilingOutputDir,
		EnableGoPs:             DefaultEnableGoPs,
		GoPsAddr:               DefaultGoPsAddr,
		EnableHTTPPprof:        DefaultEnableHTTPPprof,
		EnableStatsViz:         DefaultEnableStatsViz,
	}
}

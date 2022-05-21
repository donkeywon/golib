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
	EnableStartupProfiling bool   `yaml:"enableStartupProfiling"   env:"PROF_ENABLE_STARTUP_PROFILING"   flag-long:"prof-enable-startup-profiling" flag-description:"profiling at startup"`
	StartupProfilingSec    int    `yaml:"startupProfilingSec"      env:"PROF_STARTUP_PROFILING_SEC"      flag-long:"prof-startup-profiling-sec"    flag-description:"startup profiling duration in seconds, only works when prof-enable-startup-profiling is enabled"`
	StartupProfilingMode   string `yaml:"startupProfilingMode"     env:"PROF_STARTUP_PROFILING_MODE"     flag-long:"prof-startup-profiling-mode"   flag-description:"startup profiling mode, only works when prof-enable-startup-profiling is enabled"`
	ProfOutputDir          string `yaml:"profOutputDir"            env:"PROF_OUTPUT_DIR"                 flag-long:"prof-output-dir"               flag-description:"dir path of pprof file save to"`

	EnableHTTPProf       bool   `yaml:"enableHTTPProf" env:"PROF_ENABLE_HTTP_PROF" flag-long:"prof-enable-http-prof" flag-description:"enable prof over http, depends on httpd"`
	EnableWebProf        bool   `yaml:"enableWebProf"  env:"PROF_ENABLE_WEB_PROF"  flag-long:"prof-enable-web-prof"  flag-description:"enable prof over web, depends on httpd"`
	EnableWebPrettyTrace bool   `yaml:"enableWebPrettyTrace" env:"PROF_ENABLE_WEB_PRETTY_TRACE" flag-long:"prof-enable-web-pretty-trace" flag-description:"enable pretty trace over web, depends on httpd"`
	WebAuthUser          string `yaml:"webAuthUser" env:"PROF_WEB_AUTH_USER" flag-long:"prof-web-auth-user" flag-description:"web auth username"`
	WebAuthPwd           string `yaml:"webAuthPwd" env:"PROF_WEB_AUTH_PWD" flag-long:"prof-web-auth-pwd" flag-description:"web auth password"`

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
		EnableHTTPProf:         DefaultEnableHTTPProf,
		EnableWebProf:          DefaultEnableWebProf,
		EnableWebPrettyTrace:   DefaultEnableWebPrettyTrace,
		EnableStatsViz:         DefaultEnableStatsViz,
	}
}

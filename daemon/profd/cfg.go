package profd

const (
	DefaultEnableGoPs           = false
	DefaultGoPsAddr             = ":"
	DefaultEnableWebProf        = false
	DefaultEnableWebPrettyTrace = false
	DefaultEnableStatsViz       = false
	DefaultHTTPURLPrefix        = ""
)

type Cfg struct {
	EnableWebProf        bool   `yaml:"enableWebProf"        env:"ENABLE_WEB_PROF"         long:"enable-web-prof"         description:"enable prof over web, depends on httpd"`
	EnableWebPrettyTrace bool   `yaml:"enableWebPrettyTrace" env:"ENABLE_WEB_PRETTY_TRACE" long:"enable-web-pretty-trace" description:"enable pretty trace over web, depends on httpd"`
	WebAuthUser          string `yaml:"webAuthUser"          env:"WEB_AUTH_USER"`
	WebAuthPwd           string `yaml:"webAuthPwd"           env:"WEB_AUTH_PWD"`
	HTTPURLPrefix        string `yaml:"httpUrlPrefix"        env:"HTTP_URL_PREFIX"         long:"http-url-prefix"         description:"http url prefix"`

	EnableGoPs bool   `yaml:"enableGoPs" env:"ENABLE_GOPS" long:"enable-gops" description:"enable gops agent"`
	GoPsAddr   string `yaml:"goPsAddr"   env:"GOPS_ADDR"   long:"gops-addr"   description:"gops agent listen addr"`

	EnableStatsViz bool `yaml:"enableStatsViz" env:"ENABLE_STATS_VIZ" long:"enable-stats-viz" description:"enable statsviz, need httpd"`
}

func NewCfg() Cfg {
	return Cfg{
		EnableGoPs:           DefaultEnableGoPs,
		GoPsAddr:             DefaultGoPsAddr,
		EnableWebProf:        DefaultEnableWebProf,
		EnableWebPrettyTrace: DefaultEnableWebPrettyTrace,
		EnableStatsViz:       DefaultEnableStatsViz,
		HTTPURLPrefix:        DefaultHTTPURLPrefix,
	}
}

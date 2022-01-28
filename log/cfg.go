package log

const (
	FilepathSplitter       = ","
	DefaultFilepath        = "stdout"
	DefaultMaxFileSize     = 100
	DefaultMaxBackups      = 30
	DefaultMaxAge          = 30
	DefaultDisableCompress = false
)

// Cfg is logger cfg include level, rotate, etc.
type Cfg struct {
	Filepath        string `env:"LOG_PATH"             flag-long:"log-path"             yaml:"filepath"        flag-description:"log file path"`
	Format          string `env:"LOG_FORMAT"           flag-long:"log-format"           yaml:"format"          flag-description:"log line format"`
	MaxFileSize     int    `env:"LOG_MAX_FILE_SIZE"    flag-long:"log-max-file-size"    yaml:"maxFileSize"     flag-description:"maximum size in megabytes of the log file before it gets rotated"`
	MaxBackups      int    `env:"LOG_MAX_BACKUPS"      flag-long:"log-max-backups"      yaml:"maxBackups"      flag-description:"maximum number of old log files to retain"`
	MaxAge          int    `env:"LOG_MAX_AGE"          flag-long:"log-max-age"          yaml:"maxAge"          flag-description:"maximum number of days to retain old log files based on the timestamp encoded in their filename"`
	Level           string `env:"LOG_LEVEL"            flag-long:"log-level"            yaml:"level"           flag-description:"minimum enabled logging level"`
	DisableCompress bool   `env:"LOG_DISABLE_COMPRESS" flag-long:"log-disable-compress" yaml:"disableCompress" flag-description:"disable compress using gzip after log rotate"`
}

func NewCfg() *Cfg {
	return &Cfg{
		Level:           DefaultLevel,
		Filepath:        DefaultFilepath,
		MaxFileSize:     DefaultMaxFileSize,
		MaxBackups:      DefaultMaxBackups,
		MaxAge:          DefaultMaxAge,
		DisableCompress: DefaultDisableCompress,
		Format:          DefaultFormat,
	}
}

// Build logger from cfg
// DO NOT CREATE GLOBAL LOGGER.
// USE PROVIDED LOGGER, SUCH AS Runner.Info
func (c *Cfg) Build() (Logger, error) {
	return NewZapLogger(c)
}

func Default() Logger {
	l, _ := NewCfg().Build()
	return l
}

func Debug() Logger {
	lc := NewCfg()
	lc.Level = "debug"
	l, _ := lc.Build()
	return l
}

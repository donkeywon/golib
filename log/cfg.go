package log

const (
	FilepathSplitter      = ","
	DefaultFilepath       = "stdout"
	DefaultMaxFileSize    = 100
	DefaultMaxBackups     = 30
	DefaultMaxAge         = 30
	DefaultEnableCompress = false
	DefaultCompression    = "zstd"
)

// Cfg is logger cfg include level, rotate, etc.
type Cfg struct {
	Filepath       string `env:"FILEPATH"        long:"filepath"        yaml:"filepath"       description:"log file path"`
	Format         string `env:"FORMAT"          long:"format"          yaml:"format"         description:"log line format"`
	MaxFileSize    int    `env:"MAX_FILE_SIZE"   long:"max-file-size"   yaml:"maxFileSize"    description:"maximum size in megabytes of the log file before it gets rotated"`
	MaxBackups     int    `env:"MAX_BACKUPS"     long:"max-backups"     yaml:"maxBackups"     description:"maximum number of old log files to retain"`
	MaxAge         int    `env:"MAX_AGE"         long:"max-age"         yaml:"maxAge"         description:"maximum number of days to retain old log files based on the timestamp encoded in their filename"`
	Level          string `env:"LEVEL"           long:"level"           yaml:"level"          description:"minimum enabled logging level"`
	EnableCompress bool   `env:"ENABLE_COMPRESS" long:"enable-compress" yaml:"enableCompress" description:"enable compress using gzip after log rotate"`
	Compression    string `env:"COMPRESSION"     long:"compression"     yaml:"compression"    description:"gzip or zstd"`
}

func NewCfg() *Cfg {
	return &Cfg{
		Level:          DefaultLevel,
		Filepath:       DefaultFilepath,
		MaxFileSize:    DefaultMaxFileSize,
		MaxBackups:     DefaultMaxBackups,
		MaxAge:         DefaultMaxAge,
		EnableCompress: DefaultEnableCompress,
		Format:         DefaultFormat,
		Compression:    DefaultCompression,
	}
}

// Build logger from cfg.
// DO NOT CREATE GLOBAL LOGGER.
// USE PROVIDED LOGGER, SUCH AS Runner.Info
func (c *Cfg) Build() (Logger, error) {
	return NewZapLogger(c)
}

func Default() Logger {
	l, _ := NewCfg().Build()
	return l
}

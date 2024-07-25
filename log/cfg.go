package log

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/donkeywon/golib/log/core"
	"github.com/donkeywon/golib/util"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	FilepathSplitter   = ","
	DefaultFilepath    = "stdout"
	DefaultMaxFileSize = 100
	DefaultMaxBackups  = 30
	DefaultMaxAge      = 30
	DefaultCompress    = true
)

type Cfg struct {
	Filepath    string        `env:"LOG_PATH"          flag-long:"log-path"          yaml:"filepath"    flag-description:"log file path"`
	Format      string        `env:"LOG_FORMAT"        flag-long:"log-format"        yaml:"format"      flag-description:"log line format, console or json"`
	MaxFileSize int           `env:"LOG_MAX_FILE_SIZE" flag-long:"log-max-file-size" yaml:"maxFileSize" flag-description:"maximum size in megabytes of the log file before it gets rotated"`
	MaxBackups  int           `env:"LOG_MAX_BACKUPS"   flag-long:"log-max-backups"   yaml:"maxBackups"  flag-description:"maximum number of old log files to retain"`
	MaxAge      int           `env:"LOG_MAX_AGE"       flag-long:"log-max-age"       yaml:"maxAge"      flag-description:"maximum number of days to retain old log files based on the timestamp encoded in their filename"`
	Level       zapcore.Level `env:"LOG_LEVEL"         flag-long:"log-level"         yaml:"level"       flag-description:"minimum enabled logging level"`
	Compress    bool          `env:"LOG_COMPRESS"      flag-long:"log-compress"      yaml:"compress"    flag-description:"determines if the rotated log files should be compressed using gzip"`
}

func NewCfg() *Cfg {
	return &Cfg{
		Level:       DefaultLevel,
		Filepath:    DefaultFilepath,
		MaxFileSize: DefaultMaxFileSize,
		MaxBackups:  DefaultMaxBackups,
		MaxAge:      DefaultMaxAge,
		Compress:    DefaultCompress,
		Format:      DefaultFormat,
	}
}

func (c *Cfg) Build(opts ...zap.Option) (*zap.Logger, error) {
	var err error
	cfg := DefaultConfig()
	cfg.Level = zap.NewAtomicLevelAt(c.Level)
	cfg.Encoding = c.Format
	cfg.OutputPaths, err = c.buildOutputs()
	if err != nil {
		return nil, err
	}
	return cfg.Build(append(opts, zap.WrapCore(core.NewStackExtractCore), zap.AddCaller(), zap.AddCallerSkip(1))...)
}

func (c *Cfg) buildOutputs() ([]string, error) {
	paths := util.Unique(strings.Split(c.Filepath, FilepathSplitter))
	var outputs []string
	for _, fp := range paths {
		fp = strings.TrimSpace(fp)
		fpl := strings.ToLower(fp)
		switch fpl {
		case "stdout":
			outputs = append(outputs, "stdout")
		case "stderr":
			outputs = append(outputs, "stderr")
		default:
			if !util.DirExist(filepath.Dir(fp)) {
				return nil, errors.New("log dir not exists: " + fp)
			}
			lj := &lumberjack.Logger{
				Filename:   fp,
				MaxSize:    c.MaxFileSize,
				MaxBackups: c.MaxBackups,
				MaxAge:     c.MaxAge,
				Compress:   c.Compress,
				LocalTime:  true,
			}
			bs, err := json.Marshal(lj)
			if err != nil {
				return nil, errors.New("lumberjack config invalid")
			}
			outputs = append(outputs, "lumberjack://"+fp+"?"+string(bs))
		}
	}
	return outputs, nil
}

func Default(option ...zap.Option) *zap.Logger {
	l, _ := NewCfg().Build(option...)
	return l
}

func Debug(option ...zap.Option) *zap.Logger {
	lc := NewCfg()
	lc.Level = zap.DebugLevel
	l, _ := lc.Build(append(option, zap.Development())...)
	return l
}

package log

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/log/core"
	"github.com/donkeywon/golib/util"

	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	FilepathSplitter       = ","
	DefaultFilepath        = "stdout"
	DefaultMaxFileSize     = 100
	DefaultMaxBackups      = 30
	DefaultMaxAge          = 30
	DefaultDisableCompress = false
)

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

func (c *Cfg) Build(opts ...zap.Option) (*zap.Logger, error) {
	var err error
	cfg := DefaultConfig()
	cfg.Level, err = zap.ParseAtomicLevel(c.Level)
	if err != nil {
		return nil, errs.Wrap(err, "invalid log level")
	}
	cfg.Encoding = c.Format
	cfg.OutputPaths, err = c.buildOutputs()
	if err != nil {
		return nil, errs.Wrap(err, "build log outputs fail")
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
				Compress:   !c.DisableCompress,
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
	lc.Level = "debug"
	l, _ := lc.Build(append(option, zap.Development())...)
	return l
}

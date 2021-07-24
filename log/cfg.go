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
	Filepath    string        `env:"LOG_PATH"          yaml:"filepath"`
	Encoding    string        `env:"LOG_ENCODING"      yaml:"encoding"`
	MaxFileSize int           `env:"LOG_MAX_FILE_SIZE" yaml:"maxFileSize"`
	MaxBackups  int           `env:"LOG_MAX_BACKUPS"   yaml:"maxBackups"`
	MaxAge      int           `env:"LOG_MAX_AGE"       yaml:"maxAge"`
	Level       zapcore.Level `env:"LOG_LEVEL"         yaml:"level"`
	Compress    bool          `env:"LOG_COMPRESS"      yaml:"compress"`
}

func NewCfg() *Cfg {
	return &Cfg{
		Level:       DefaultLevel,
		Filepath:    DefaultFilepath,
		MaxFileSize: DefaultMaxFileSize,
		MaxBackups:  DefaultMaxBackups,
		MaxAge:      DefaultMaxAge,
		Compress:    DefaultCompress,
		Encoding:    DefaultEncoding,
	}
}

func (c *Cfg) Build(opts ...zap.Option) (*zap.Logger, error) {
	var err error
	cfg := DefaultConfig()
	cfg.Level = zap.NewAtomicLevelAt(c.Level)
	cfg.Encoding = c.Encoding
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

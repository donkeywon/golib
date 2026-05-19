package logs

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/DeRuina/timberjack"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/paths"
)

type Creator interface {
	Create() (*slog.Logger, error)
}

type NopLoggerCreator struct{}

func (n *NopLoggerCreator) Create() (*slog.Logger, error) {
	return slog.New(slog.DiscardHandler), nil
}

const (
	FilepathSplitter      = ","
	DefaultFormat         = "json"
	DefaultFilepath       = "stdout"
	DefaultMaxFileSize    = 100
	DefaultMaxBackups     = 30
	DefaultMaxAge         = 30
	DefaultEnableCompress = false
	DefaultCompression    = "zstd"
)

var (
	DefaultLevel = &slog.LevelVar{}
)

type RotateLoggerCreator struct {
	Filepath       string       `env:"FILEPATH"        long:"filepath"        yaml:"filepath"       description:"log file path"`
	Format         string       `env:"FORMAT"          long:"format"          yaml:"format"         description:"log line format"`
	MaxFileSize    int          `env:"MAX_FILE_SIZE"   long:"max-file-size"   yaml:"maxFileSize"    description:"maximum size in megabytes of the log file before it gets rotated"`
	MaxBackups     int          `env:"MAX_BACKUPS"     long:"max-backups"     yaml:"maxBackups"     description:"maximum number of old log files to retain"`
	MaxAge         int          `env:"MAX_AGE"         long:"max-age"         yaml:"maxAge"         description:"maximum number of days to retain old log files based on the timestamp encoded in their filename"`
	Level          slog.Leveler `env:"LEVEL"           long:"level"           yaml:"level"          description:"minimum enabled logging level"`
	EnableCompress bool         `env:"ENABLE_COMPRESS" long:"enable-compress" yaml:"enableCompress" description:"enable compress using gzip after log rotate"`
	Compression    string       `env:"COMPRESSION"     long:"compression"     yaml:"compression"    description:"gzip or zstd"`
}

func NewRotateLoggerCreator() *RotateLoggerCreator {
	return &RotateLoggerCreator{
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

func (r *RotateLoggerCreator) Create() (*slog.Logger, error) {
	outputs, err := buildOutputs(r)
	if err != nil {
		return nil, errs.Wrapf(err, "build logger outputs failed: %s", r.Filepath)
	}
	if len(outputs) == 0 {
		return nil, errs.Errorf("empty logger outputs")
	}

	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     r.Level,
	}
	handlers := make([]slog.Handler, 0, len(outputs))
	for i := range outputs {
		if r.Format == "json" {
			handlers = append(handlers, slog.NewJSONHandler(outputs[i], opts))
		} else {
			handlers = append(handlers, slog.NewTextHandler(outputs[i], opts))
		}
	}
	return slog.New(slog.NewMultiHandler(handlers...)), nil
}

func buildOutputs(c *RotateLoggerCreator) ([]io.Writer, error) {
	fps := uniqueStrings(strings.Split(c.Filepath, FilepathSplitter))
	var outputs []io.Writer
	for _, fp := range fps {
		fp = strings.TrimSpace(fp)
		fpl := strings.ToLower(fp)
		switch fpl {
		case "stdout":
			outputs = append(outputs, os.Stdout)
		case "stderr":
			outputs = append(outputs, os.Stderr)
		default:
			if !paths.DirExist(filepath.Dir(fp)) {
				return nil, errors.New("log dir not exists: " + fp)
			}
			tj := &timberjack.Logger{
				Filename:    fp,
				MaxSize:     c.MaxFileSize,
				MaxBackups:  c.MaxBackups,
				MaxAge:      c.MaxAge,
				Compress:    c.EnableCompress,
				Compression: c.Compression,
				LocalTime:   true,
			}
			outputs = append(outputs, tj)
		}
	}
	return outputs, nil
}

func uniqueStrings(strs []string) []string {
	if len(strs) <= 1 {
		return strs
	}

	result := make([]string, 0, len(strs))
	seen := make(map[string]struct{}, len(strs))

	for _, str := range strs {
		if _, ok := seen[str]; ok {
			continue
		}

		seen[str] = struct{}{}
		result = append(result, str)
	}

	return result
}

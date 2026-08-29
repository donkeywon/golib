package logs

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DeRuina/timberjack"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util"
	"github.com/donkeywon/golib/util/paths"
	"github.com/petermattis/goid"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		keep := 2
		for i := len(file) - 1; i >= 0; i-- {
			if file[i] == '/' {
				keep--
				if keep == 0 {
					file = file[i+1:]
					break
				}
			}
		}
		return file + ":" + strconv.Itoa(line)
	}
	zerolog.ErrorMarshalFunc = func(err error) any {
		return errs.ErrToStackString(err)
	}
}

var NopLoggerCreator = nop{}

type nop struct{}

func (n nop) Create() (zerolog.Logger, error) {
	return zerolog.Nop(), nil
}

const (
	FilepathSplitter   = ","
	DefaultFilepath    = "stderr"
	DefaultMaxFileSize = 100
	DefaultMaxBackups  = 30
	DefaultMaxAge      = 30
	DefaultCompression = "zstd"
)

type RotateLoggerCreator struct {
	Level       zerolog.Level `env:"LEVEL"         long:"level"         yaml:"level"       description:"minimum enabled logging level"`
	Filepath    string        `env:"FILEPATH"      long:"filepath"      yaml:"filepath"    description:"log file path"`
	MaxFileSize int           `env:"MAX_FILE_SIZE" long:"max-file-size" yaml:"maxFileSize" description:"maximum size in megabytes of the log file before it gets rotated"`
	MaxBackups  int           `env:"MAX_BACKUPS"   long:"max-backups"   yaml:"maxBackups"  description:"maximum number of old log files to retain"`
	MaxAge      int           `env:"MAX_AGE"       long:"max-age"       yaml:"maxAge"      description:"maximum number of days to retain old log files based on the timestamp encoded in their filename"`
	Compression string        `env:"COMPRESSION"   long:"compression"   yaml:"compression" description:"none,gzip,zstd"`
}

func NewRotateLoggerCreator() RotateLoggerCreator {
	return RotateLoggerCreator{
		Level:       zerolog.InfoLevel,
		Filepath:    DefaultFilepath,
		MaxFileSize: DefaultMaxFileSize,
		MaxBackups:  DefaultMaxBackups,
		MaxAge:      DefaultMaxAge,
		Compression: DefaultCompression,
	}
}

func (r RotateLoggerCreator) Create() (zerolog.Logger, error) {
	outputs, err := buildOutputs(&r)
	if err != nil {
		return zerolog.Nop(), errs.Wrapf(err, "build logger outputs failed: %s", r.Filepath)
	}
	if len(outputs) == 0 {
		return zerolog.Nop(), errs.Errorf("empty logger outputs")
	}

	l := zerolog.New(zerolog.MultiLevelWriter(outputs...)).
		With().Caller().Timestamp().Logger().Level(r.Level).
		Hook(zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			e.Int64("goid", goid.Get())
		}))
	return l, nil
}

func buildOutputs(c *RotateLoggerCreator) ([]io.Writer, error) {
	fps := util.Unique(strings.Split(c.Filepath, FilepathSplitter))
	var outputs []io.Writer
	for _, fp := range fps {
		fp = strings.TrimSpace(fp)
		fpl := strings.ToLower(fp)
		switch fpl {
		case "stdout":
			outputs = append(outputs, zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
				w.Out = os.Stdout
				w.TimeFormat = "0102 15:04:05.000000"
			}))
		case "stderr":
			outputs = append(outputs, zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
				w.Out = os.Stderr
				w.TimeFormat = "0102 15:04:05.000000"
			}))
		default:
			if !paths.DirExist(filepath.Dir(fp)) {
				return nil, errs.New("log dir not exists: " + fp)
			}
			tj := &timberjack.Logger{
				Filename:    fp,
				MaxSize:     c.MaxFileSize,
				MaxBackups:  c.MaxBackups,
				MaxAge:      c.MaxAge,
				Compression: c.Compression,
				LocalTime:   true,
			}
			outputs = append(outputs, tj)
		}
	}
	return outputs, nil
}

package log

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/log/core"
	"github.com/donkeywon/golib/log/encoder"
	"github.com/donkeywon/golib/log/sink"
	"github.com/donkeywon/golib/util"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/paths"
	"github.com/petermattis/goid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func init() {
	// lumberjack:///var/log/xxx.log?{"maxsize":100,"maxage":30,"maxbackups":30,"compress":true,"localtime":false}
	_ = zap.RegisterSink("lumberjack", sink.NewLumberJackSinkFromURL)

	_ = zap.RegisterEncoder("consolelite", encoder.NewConsoleLiteEncoder)
}

const (
	JSONFormat        = "json"
	ConsoleFormat     = "console"
	ConsoleLiteFormat = "consolelite"

	LevelEncoderCapital      = "capital"
	LevelEncoderCapitalColor = "capitalColor"
	LevelEncoderColor        = "color"
	LevelEncoderLowercase    = "lowercase"

	TimeEncoderRFC3339Nano = "rfc3339nano"
	TimeEncoderRFC3339     = "rfc3339"
	TimeEncoderISO8601     = "iso8601"
	TimeEncoderMillis      = "millis"
	TimeEncoderNanos       = "nanos"
	TimeEncoderSecond      = "second"
	TimeEncoderLayout      = "2006-01-02 15:04:05.000000"

	DurationEncoderString = "string"
	DurationEncoderNanos  = "nanos"
	DurationEncoderMillis = "ms"
	DurationEncoderSecond = "second"

	CallerEncoderFull  = "full"
	CallerEncoderShort = "short"

	NameEncoderFull = "full"

	DefaultLevel             = "info"
	DefaultIsDev             = false
	DefaultDisableCaller     = false
	DefaultDisableStacktrace = true // stack core will extract error stack, so zap's stack is useless

	DefaultFormat                  = ConsoleLiteFormat
	DefaultEncoderMessageKey       = "msg"
	DefaultEncoderLevelKey         = "lvl"
	DefaultEncoderNameKey          = "logger"
	DefaultEncoderTimeKey          = "ts"
	DefaultEncoderCallerKey        = "caller"
	DefaultEncoderFunctionKey      = ""
	DefaultEncoderStacktraceKey    = "stacktrace"
	DefaultEncoderSkipLineEncoding = false
	DefaultEncoderLineEnding       = "\n"
	DefaultEncoderLevelEncoder     = LevelEncoderLowercase
	DefaultEncoderTimeEncoder      = TimeEncoderLayout
	DefaultEncoderDurationEncoder  = DurationEncoderString
	DefaultEncoderCallerEncoder    = CallerEncoderShort
	DefaultEncoderNameEncoder      = NameEncoderFull
	DefaultEncoderConsoleSeparator = "\t"
)

var (
	DefaultOutputPath      = []string{"stdout"}
	DefaultErrorOutputPath = []string{"stderr"}
)

func buildTimeEncoder(enc string) zapcore.TimeEncoder {
	var te zapcore.TimeEncoder
	if enc[0] >= '0' && enc[0] <= '9' {
		te = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString(ts.Format(enc))
		}
	} else {
		_ = te.UnmarshalText([]byte(enc))
	}

	return te
}

func HandleZapFields(withGoid bool, args []any, additional ...zap.Field) []zap.Field {
	if len(args) == 0 {
		if !withGoid {
			return additional
		}
		return append(additional, zap.Int64("goid", goid.Get()))
	}

	var fields []zap.Field
	if withGoid {
		fields = make([]zap.Field, 0, len(args)/2+len(additional)+1)
	} else {
		fields = make([]zap.Field, 0, len(args)/2+len(additional))
	}

	for i := 0; i < len(args); i += 2 {
		switch k := args[i].(type) {
		case string:
			if i == len(args)-1 {
				fields = append(fields, zap.String("!BADKEY", k))
			} else {
				switch a := args[i+1].(type) {
				case []byte:
					fields = append(fields, zap.ByteString(k, a))
				default:
					fields = append(fields, zap.Any(k, args[i+1]))
				}
			}
		case zap.Field:
			fields = append(fields, k)
			i--
		default:
			fields = append(fields, zap.Any("!BADKEY", k))
			i--
		}
	}

	if withGoid {
		fields = append(fields, additional...)
		return append(fields, zap.Int64("goid", goid.Get()))
	}

	return append(fields, additional...)
}

func DefaultEncoderConfig() zapcore.EncoderConfig {
	var le zapcore.LevelEncoder
	_ = le.UnmarshalText([]byte(DefaultEncoderLevelEncoder))

	te := buildTimeEncoder(DefaultEncoderTimeEncoder)

	var de zapcore.DurationEncoder
	_ = de.UnmarshalText([]byte(DefaultEncoderDurationEncoder))

	var ce zapcore.CallerEncoder
	_ = ce.UnmarshalText([]byte(DefaultEncoderCallerEncoder))

	var ne zapcore.NameEncoder
	_ = ne.UnmarshalText([]byte(DefaultEncoderNameEncoder))

	config := zapcore.EncoderConfig{
		MessageKey:          DefaultEncoderMessageKey,
		LevelKey:            DefaultEncoderLevelKey,
		TimeKey:             DefaultEncoderTimeKey,
		NameKey:             DefaultEncoderNameKey,
		CallerKey:           DefaultEncoderCallerKey,
		FunctionKey:         DefaultEncoderFunctionKey,
		StacktraceKey:       DefaultEncoderStacktraceKey,
		SkipLineEnding:      DefaultEncoderSkipLineEncoding,
		LineEnding:          DefaultEncoderLineEnding,
		EncodeLevel:         le,
		EncodeTime:          te,
		EncodeDuration:      de,
		EncodeCaller:        ce,
		EncodeName:          ne,
		ConsoleSeparator:    DefaultEncoderConsoleSeparator,
		NewReflectedEncoder: func(w io.Writer) zapcore.ReflectedEncoder { return jsons.NewEncoder(w) },
	}

	return config
}

func DefaultConfig() *zap.Config {
	lvl, _ := zap.ParseAtomicLevel(DefaultLevel)
	return &zap.Config{
		Level:             lvl,
		Development:       DefaultIsDev,
		DisableCaller:     DefaultDisableCaller,
		DisableStacktrace: DefaultDisableStacktrace,
		Sampling:          nil,
		Encoding:          DefaultFormat,
		EncoderConfig:     DefaultEncoderConfig(),
		OutputPaths:       DefaultOutputPath,
		ErrorOutputPaths:  DefaultErrorOutputPath,
		InitialFields:     nil,
	}
}

type zapLogger struct {
	*zap.Logger

	lvl zap.AtomicLevel
}

func NewZapLogger(c *Cfg) (Logger, error) {
	lvl, err := zap.ParseAtomicLevel(c.Level)
	if err != nil {
		return nil, errs.Wrap(err, "invalid log level")
	}

	cfg := DefaultConfig()
	cfg.Level = lvl
	cfg.Encoding = c.Format
	cfg.OutputPaths, err = buildOutputs(c)
	if err != nil {
		return nil, errs.Wrap(err, "build log outputs failed")
	}
	zl, err := cfg.Build(zap.WrapCore(core.NewStackExtractCore), zap.AddCaller(), zap.AddCallerSkip(1))
	if err != nil {
		return nil, errs.Wrap(err, "build logger failed")
	}
	return &zapLogger{
		Logger: zl,
		lvl:    lvl,
	}, nil
}

func buildOutputs(c *Cfg) ([]string, error) {
	fps := util.Unique(strings.Split(c.Filepath, FilepathSplitter))
	var outputs []string
	for _, fp := range fps {
		fp = strings.TrimSpace(fp)
		fpl := strings.ToLower(fp)
		switch fpl {
		case "stdout":
			outputs = append(outputs, "stdout")
		case "stderr":
			outputs = append(outputs, "stderr")
		default:
			if !paths.DirExist(filepath.Dir(fp)) {
				return nil, errors.New("log dir not exists: " + fp)
			}
			lj := &lumberjack.Logger{
				Filename:   fp,
				MaxSize:    c.MaxFileSize,
				MaxBackups: c.MaxBackups,
				MaxAge:     c.MaxAge,
				Compress:   c.EnableCompress,
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

func (z *zapLogger) WithLoggerName(n string) Logger {
	return &zapLogger{
		Logger: z.Logger.Named(n),
		lvl:    z.lvl,
	}
}

func (z *zapLogger) WithLoggerFields(kvs ...any) {
	z.Logger = z.Logger.With(HandleZapFields(false, kvs)...)
}

func (z *zapLogger) SetLoggerLevel(lvl string) {
	z.lvl.UnmarshalText([]byte(lvl))
}

func (z *zapLogger) Debug(msg string, kvs ...any) {
	z.Logger.Debug(msg, HandleZapFields(true, kvs)...)
}

func (z *zapLogger) Info(msg string, kvs ...any) {
	z.Logger.Info(msg, HandleZapFields(true, kvs)...)
}

func (z *zapLogger) Warn(msg string, kvs ...any) {
	z.Logger.Warn(msg, HandleZapFields(true, kvs)...)
}

func (z *zapLogger) Error(msg string, err error, kvs ...any) {
	z.Logger.Error(msg, HandleZapFields(true, kvs, zap.Error(err))...)
}

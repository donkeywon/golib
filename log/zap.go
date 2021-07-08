package log

import (
	"io"
	"time"

	"github.com/bytedance/sonic/encoder"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	JSONEncoding    = "json"
	ConsoleEncoding = "console"

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

	DefaultLevel             = zap.InfoLevel
	DefaultIsDev             = false
	DefaultDisableCaller     = false
	DefaultDisableStacktrace = true // stack core will extract error stack, so zap's stack is useless

	DefaultEncoding                = ConsoleEncoding
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

func HandleZapFields(args []interface{}, additional ...zap.Field) []zap.Field {
	if len(args) == 0 {
		return additional
	}

	fields := make([]zap.Field, 0, len(args)/2+len(additional))
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
		NewReflectedEncoder: sonicReflectEncoder,
	}

	return config
}

func sonicReflectEncoder(w io.Writer) zapcore.ReflectedEncoder {
	return encoder.NewStreamEncoder(w)
}

func DefaultConfig() *zap.Config {
	return &zap.Config{
		Level:             zap.NewAtomicLevelAt(DefaultLevel),
		Development:       DefaultIsDev,
		DisableCaller:     DefaultDisableCaller,
		DisableStacktrace: DefaultDisableStacktrace,
		Sampling:          nil,
		Encoding:          DefaultEncoding,
		EncoderConfig:     DefaultEncoderConfig(),
		OutputPaths:       DefaultOutputPath,
		ErrorOutputPaths:  DefaultErrorOutputPath,
		InitialFields:     nil,
	}
}

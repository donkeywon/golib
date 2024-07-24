package log

import (
	"go.uber.org/zap"
)

// Logger is a log interface.
// DO NOT CREATE GLOBAL LOGGER.
// USE PROVIDED LOG METHOD, SUCH AS Runner.Info.
type Logger interface {
	Debug(msg string, kvs ...any)
	Info(msg string, kvs ...any)
	Warn(msg string, kvs ...any)
	Error(msg string, err error, kvs ...any)

	WithLoggerName(n string) Logger
	WithLoggerFields(kvs ...any)
	SetLogLevel(lvl string)
}

func NewNopLogger() Logger {
	return &zapLogger{Logger: zap.NewNop()}
}

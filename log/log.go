package log

import "go.uber.org/zap"

type Logger interface {
	Debug(msg string, kvs ...any)
	Info(msg string, kvs ...any)
	Warn(msg string, kvs ...any)
	Error(msg string, err error, kvs ...any)

	WithLoggerName(n string) Logger
	WithLoggerFields(kvs ...any)
}

func NewNopLogger() Logger {
	return &zapLogger{Logger: zap.NewNop()}
}

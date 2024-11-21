package log

type Logger interface {
	Debug(msg string, kvs ...any)
	Info(msg string, kvs ...any)
	Warn(msg string, kvs ...any)
	Error(msg string, err error, kvs ...any)
}

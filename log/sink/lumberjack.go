package sink

import (
	"net/url"

	"github.com/bytedance/sonic"
	"github.com/donkeywon/golib/errs"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

func init() {
	// lumberjack:///var/log/xxx.log?{"maxsize":100,"maxage":30,"maxbackups":30,"compress":true,"localtime",false}
	_ = zap.RegisterSink("lumberjack", newLumberJackSinkFromURL)
}

const (
	DefaultMaxFileSize = 100
	DefaultMaxBackups  = 30
	DefaultMaxAge      = 30
	DefaultCompress    = true
)

type lumberjackSink struct {
	*lumberjack.Logger
}

func (ls *lumberjackSink) Sync() error {
	return nil
}

func newLumberJackSinkFromURL(u *url.URL) (zap.Sink, error) {
	l, err := pathToLogger(u.Path, u.RawQuery)
	if err != nil {
		return nil, errs.Wrap(err, "lumberjack sink config invalid")
	}

	return &lumberjackSink{l}, nil
}

func pathToLogger(path string, cfg string) (*lumberjack.Logger, error) {
	l := &lumberjack.Logger{
		Filename:   "",
		MaxSize:    DefaultMaxFileSize,
		MaxAge:     DefaultMaxAge,
		MaxBackups: DefaultMaxBackups,
		LocalTime:  true,
		Compress:   DefaultCompress,
	}
	err := sonic.Unmarshal([]byte(cfg), l)
	l.Filename = path
	return l, err
}

package sink

import (
	"encoding/json"
	"net/url"

	"github.com/DeRuina/timberjack"
	"github.com/donkeywon/golib/errs"
	"go.uber.org/zap"
)

const (
	DefaultCompression = "zstd"
)

type timberjackSink struct {
	*timberjack.Logger
}

func (ts *timberjackSink) Sync() error { return nil }

func NewTimberJackSinkFromURL(u *url.URL) (zap.Sink, error) {
	t, err := pathToTimberJackLogger(u.Path, u.RawQuery)
	if err != nil {
		return nil, errs.Wrap(err, "invalid timberjack sink config")
	}
	return &timberjackSink{t}, nil
}

func pathToTimberJackLogger(path string, cfg string) (*timberjack.Logger, error) {
	l := &timberjack.Logger{
		Filename:    "",
		MaxSize:     DefaultMaxFileSize,
		MaxAge:      DefaultMaxAge,
		MaxBackups:  DefaultMaxBackups,
		LocalTime:   true,
		Compression: DefaultCompression,
	}
	err := json.Unmarshal([]byte(cfg), l)
	l.Filename = path
	return l, err
}

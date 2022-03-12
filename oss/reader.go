package oss

import (
	"context"
	"net/http"
	"time"

	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/httpio/reader"
	"github.com/donkeywon/golib/util/oss"
)

type Reader struct {
	*Cfg
	*reader.Reader
}

func NewReader(ctx context.Context, cfg *Cfg) *Reader {
	r := &Reader{
		Cfg: cfg,
	}
	r.Reader = reader.New(ctx,
		time.Second*time.Duration(cfg.Timeout),
		cfg.URL,
		reader.Retry(cfg.Retry),
		reader.WithReqOptions(
			httpc.ReqOptionFunc(func(r *http.Request) error {
				return oss.Sign(r, cfg.Ak, cfg.Sk, cfg.Region)
			}),
		),
	)
	return r
}

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

func NewReader(ctx context.Context, cfg *Cfg, opts ...reader.Option) *Reader {
	r := &Reader{
		Cfg: cfg,
	}
	allOpts := make([]reader.Option, 0, 2+len(opts))
	allOpts = append(allOpts,
		reader.Retry(cfg.Retry),
		reader.WithReqOptions(
			httpc.ReqOptionFunc(func(r *http.Request) error {
				return oss.Sign(r, cfg.Ak, cfg.Sk, cfg.Region)
			}),
		))
	allOpts = append(allOpts, opts...)

	r.Reader = reader.New(ctx,
		time.Second*time.Duration(cfg.Timeout),
		cfg.URL,
		allOpts...,
	)
	return r
}

package oss

import (
	"context"
	"net/http"
	"time"

	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/httpio"
	"github.com/donkeywon/golib/util/oss"
)

type Reader struct {
	*httpio.Reader
	cfg *Cfg
}

func NewReader(ctx context.Context, cfg *Cfg, opts ...httpc.Option) *Reader {
	r := &Reader{
		cfg: cfg,
	}
	cfg.setDefaults()
	allHttpcOptions := make([]httpc.Option, 0, 1+len(opts))
	allHttpcOptions = append(allHttpcOptions, httpc.ReqOptionFunc(func(r *http.Request) error {
		return oss.Sign(r, cfg.Ak, cfg.Sk, cfg.Region)
	}))
	allHttpcOptions = append(allHttpcOptions, opts...)

	allOptions := make([]httpio.Option, 0, 5)
	allOptions = append(allOptions,
		httpio.Offset(cfg.Offset),
		httpio.PartSize(cfg.PartSize),
		httpio.Retry(cfg.Retry),
		httpio.WithHTTPOptions(allHttpcOptions...),
	)
	if cfg.NoRange {
		allOptions = append(allOptions, httpio.NoRange())
	}

	r.Reader = httpio.NewReader(ctx,
		time.Second*time.Duration(cfg.Timeout),
		cfg.URL,
		allOptions...,
	)
	return r
}

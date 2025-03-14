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
	allOpts := make([]httpc.Option, 0, 1+len(opts))
	allOpts = append(allOpts, httpc.ReqOptionFunc(func(r *http.Request) error {
		return oss.Sign(r, cfg.Ak, cfg.Sk, cfg.Region)
	}))
	allOpts = append(allOpts, opts...)

	r.Reader = httpio.NewReader(ctx,
		time.Second*time.Duration(cfg.Timeout),
		cfg.URL,
		httpio.Offset(cfg.Offset),
		httpio.PartSize(cfg.PartSize),
		httpio.Retry(cfg.Retry),
		httpio.WithHTTPOptions(allOpts...),
	)
	return r
}

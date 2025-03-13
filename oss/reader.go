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
	*Cfg
	*httpio.Reader
}

func NewReader(ctx context.Context, cfg *Cfg, opts ...httpc.Option) *Reader {
	r := &Reader{
		Cfg: cfg,
	}
	rc := &httpio.Cfg{
		Retry:    cfg.Retry,
		BeginPos: cfg.BeginPos,
	}
	allOpts := make([]httpc.Option, 0, 1+len(opts))
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts,
		httpc.ReqOptionFunc(func(r *http.Request) error {
			return oss.Sign(r, cfg.Ak, cfg.Sk, cfg.Region)
		}),
	)

	r.Reader = httpio.New(ctx,
		time.Second*time.Duration(cfg.Timeout),
		cfg.URL,
		rc,
		allOpts...,
	)
	return r
}

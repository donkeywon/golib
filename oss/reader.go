package oss

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/httpio"
	"github.com/donkeywon/golib/util/oss"
)

type Reader struct {
	*Cfg
	io.ReadCloser
}

func NewReader(ctx context.Context, cfg *Cfg, opts ...httpc.Option) *Reader {
	r := &Reader{
		Cfg: cfg,
	}
	allOpts := make([]httpc.Option, 0, 1+len(opts))
	allOpts = append(allOpts, httpc.ReqOptionFunc(func(r *http.Request) error {
		return oss.Sign(r, cfg.Ak, cfg.Sk, cfg.Region)
	}))
	allOpts = append(allOpts, opts...)

	r.ReadCloser = httpio.NewReader(ctx,
		time.Second*time.Duration(cfg.Timeout),
		cfg.URL,
		httpio.Offset(cfg.Offset),
		httpio.Retry(cfg.Retry),
		httpio.WithHTTPOptions(allOpts...),
	)
	return r
}

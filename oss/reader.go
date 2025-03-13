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
	io.Reader
}

func NewReader(ctx context.Context, cfg *Cfg, opts ...httpc.Option) *Reader {
	r := &Reader{
		Cfg: cfg,
	}
	allOpts := make([]httpc.Option, 0, 3+len(opts))
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts,
		httpio.BeginPos(cfg.BeginPos),
		httpio.Retry(cfg.Retry),
		httpio.WithHTTPOptions(httpc.ReqOptionFunc(func(r *http.Request) error {
			return oss.Sign(r, cfg.Ak, cfg.Sk, cfg.Region)
		})),
	)

	r.Reader = httpio.NewReader(ctx,
		time.Second*time.Duration(cfg.Timeout),
		cfg.URL,
		httpio.BeginPos(cfg.BeginPos),
		httpio.Retry(cfg.Retry),
		httpio.WithHTTPOptions(allOpts...),
	)
	return r
}

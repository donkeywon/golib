package oss

import (
	"context"
	"net/http"

	"github.com/donkeywon/golib/httpio"
	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/ossu"
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
		return ossu.Sign(r, cfg.Ak, cfg.Sk, cfg.Region)
	}))
	allHttpcOptions = append(allHttpcOptions, opts...)

	r.Reader = httpio.NewReader(ctx,
		cfg.URL,
		httpio.Offset(cfg.Offset),
		httpio.Retry(cfg.Retry),
		httpio.WithResponseHeaderTimeout(cfg.Timeout),
		httpio.WithHTTPOptions(allHttpcOptions...),
	)
	return r
}

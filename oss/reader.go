package oss

import (
	"context"
	"net/http"

	"codeberg.org/donkeywon/golib/httpio"
	"codeberg.org/donkeywon/golib/util/httpc"
	"codeberg.org/donkeywon/golib/util/ossu"
)

type Reader struct {
	*httpio.Reader
}

func NewReader(ctx context.Context, cfg Cfg, opts ...httpc.Option) *Reader {
	r := &Reader{}
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

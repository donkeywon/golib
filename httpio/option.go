package httpio

import (
	"net/http"
	"time"

	"github.com/donkeywon/golib/util/httpc"
)

type option struct {
	offset                int64
	limit                 int64
	retry                 int
	client                *http.Client
	responseHeaderTimeout time.Duration
	httpOptions           []httpc.Option
}

func newOption() *option {
	return &option{
		retry: 1,
	}
}

type Option func(*option)

func Offset(offset int64) Option {
	return func(o *option) {
		o.offset = offset
	}
}

func Limit(n int64) Option {
	return func(o *option) {
		o.limit = n
	}
}

func Retry(retry int) Option {
	return func(o *option) {
		if retry > 0 {
			o.retry = retry
		}
	}
}

func WithClient(c *http.Client) Option {
	return func(o *option) {
		if c != nil {
			o.client = c
		}
	}
}

func WithResponseHeaderTimeout(responseHeaderTimeout time.Duration) Option {
	return func(o *option) {
		if responseHeaderTimeout >= 0 {
			o.responseHeaderTimeout = responseHeaderTimeout
		}
	}
}

func WithHTTPOptions(opts ...httpc.Option) Option {
	return func(o *option) {
		o.httpOptions = append(o.httpOptions, opts...)
	}
}

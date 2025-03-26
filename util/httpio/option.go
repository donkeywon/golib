package httpio

import (
	"github.com/donkeywon/golib/util/httpc"
)

type option struct {
	offset      int64
	limit       int64
	partSize    int64
	retry       int
	noRange     bool
	httpOptions []httpc.Option
}

func newOption() *option {
	return &option{
		retry:    1,
		partSize: 1024 * 1024,
	}
}

type Option func(*option)

func (o Option) apply(r *option) {
	o(r)
}

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

func PartSize(s int64) Option {
	return func(o *option) {
		if s > 0 {
			o.partSize = s
		}
	}
}

func Retry(retry int) Option {
	return func(o *option) {
		if retry > 0 {
			o.retry = retry
		}
	}
}

func NoRange() Option {
	return func(o *option) {
		o.noRange = true
	}
}

func WithHTTPOptions(opts ...httpc.Option) Option {
	return func(o *option) {
		o.httpOptions = append(o.httpOptions, opts...)
	}
}

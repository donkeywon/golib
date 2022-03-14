package httpio

import (
	"github.com/donkeywon/golib/util/httpc"
)

type option struct {
	offset      int64
	n           int64
	partSize    int64
	retry       int
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
	return func(r *option) {
		r.offset = offset
	}
}

func N(n int64) Option {
	return func(r *option) {
		r.n = n
	}
}

func PartSize(s int64) Option {
	return func(r *option) {
		if s > 0 {
			r.partSize = s
		}
	}
}

func Retry(retry int) Option {
	return func(r *option) {
		if retry > 0 {
			r.retry = retry
		}
	}
}

func WithHTTPOptions(opts ...httpc.Option) Option {
	return func(r *option) {
		r.httpOptions = append(r.httpOptions, opts...)
	}
}

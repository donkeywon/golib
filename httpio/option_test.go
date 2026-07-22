package httpio

import (
	"net/http"
	"testing"
	"time"

	"github.com/donkeywon/golib/util/httpc"
	"github.com/stretchr/testify/assert"
)

func TestNewOption(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		o := newOption()
		assert.Equal(t, int64(0), o.offset)
		assert.Equal(t, int64(0), o.limit)
		assert.Equal(t, 1, o.retry)
		assert.Nil(t, o.client)
		assert.Equal(t, time.Duration(0), o.responseHeaderTimeout)
		assert.Nil(t, o.httpOptions)
	})
}

func TestOption_apply(t *testing.T) {
	t.Run("applies option to target", func(t *testing.T) {
		o := newOption()
		opt := Offset(100)
		opt.apply(o)
		assert.Equal(t, int64(100), o.offset)
	})
}

func TestOffset(t *testing.T) {
	t.Run("positive value sets offset", func(t *testing.T) {
		o := newOption()
		opt := Offset(1024)
		opt.apply(o)
		assert.Equal(t, int64(1024), o.offset)
	})

	t.Run("zero value sets offset to zero", func(t *testing.T) {
		o := newOption()
		opt := Offset(0)
		opt.apply(o)
		assert.Equal(t, int64(0), o.offset)
	})

	t.Run("negative value is allowed", func(t *testing.T) {
		o := newOption()
		opt := Offset(-1)
		opt.apply(o)
		assert.Equal(t, int64(-1), o.offset)
	})
}

func TestLimit(t *testing.T) {
	t.Run("positive value sets limit", func(t *testing.T) {
		o := newOption()
		opt := Limit(4096)
		opt.apply(o)
		assert.Equal(t, int64(4096), o.limit)
	})

	t.Run("zero value sets limit to zero", func(t *testing.T) {
		o := newOption()
		opt := Limit(0)
		opt.apply(o)
		assert.Equal(t, int64(0), o.limit)
	})

	t.Run("negative value is allowed", func(t *testing.T) {
		o := newOption()
		opt := Limit(-1)
		opt.apply(o)
		assert.Equal(t, int64(-1), o.limit)
	})
}

func TestRetry(t *testing.T) {
	t.Run("positive value overrides default retry", func(t *testing.T) {
		o := newOption()
		opt := Retry(5)
		opt.apply(o)
		assert.Equal(t, 5, o.retry)
	})

	t.Run("zero value is ignored, default kept", func(t *testing.T) {
		o := newOption()
		opt := Retry(0)
		opt.apply(o)
		assert.Equal(t, 1, o.retry)
	})

	t.Run("negative value is ignored", func(t *testing.T) {
		o := newOption()
		opt := Retry(-3)
		opt.apply(o)
		assert.Equal(t, 1, o.retry)
	})
}

func TestWithClient(t *testing.T) {
	t.Run("non-nil client sets client", func(t *testing.T) {
		o := newOption()
		c := &http.Client{}
		opt := WithClient(c)
		opt.apply(o)
		assert.Same(t, c, o.client)
	})

	t.Run("nil client is ignored", func(t *testing.T) {
		o := newOption()
		opt := WithClient(nil)
		opt.apply(o)
		assert.Nil(t, o.client)
	})
}

func TestWithResponseHeaderTimeout(t *testing.T) {
	t.Run("positive duration sets responseHeaderTimeout", func(t *testing.T) {
		o := newOption()
		d := 10 * time.Second
		opt := WithResponseHeaderTimeout(d)
		opt.apply(o)
		assert.Equal(t, d, o.responseHeaderTimeout)
	})

	t.Run("zero duration is valid and sets to zero", func(t *testing.T) {
		o := newOption()
		opt := WithResponseHeaderTimeout(0)
		opt.apply(o)
		assert.Equal(t, time.Duration(0), o.responseHeaderTimeout)
	})

	t.Run("negative duration is ignored", func(t *testing.T) {
		o := newOption()
		opt := WithResponseHeaderTimeout(-time.Second)
		opt.apply(o)
		assert.Equal(t, time.Duration(0), o.responseHeaderTimeout)
	})
}

func TestWithHTTPOptions(t *testing.T) {
	t.Run("single option appended", func(t *testing.T) {
		o := newOption()
		opt := WithHTTPOptions(httpc.WithHeaders("X-Test", "value"))
		opt.apply(o)
		assert.Len(t, o.httpOptions, 1)
	})

	t.Run("multiple options appended", func(t *testing.T) {
		o := newOption()
		opt := WithHTTPOptions(
			httpc.WithHeaders("X-A", "1"),
			httpc.WithHeaders("X-B", "2"),
		)
		opt.apply(o)
		assert.Len(t, o.httpOptions, 2)
	})

	t.Run("append to existing httpOptions", func(t *testing.T) {
		o := &option{
			retry:       1,
			httpOptions: []httpc.Option{httpc.WithHeaders("X-First", "a")},
		}
		opt := WithHTTPOptions(httpc.WithHeaders("X-Second", "b"))
		opt.apply(o)
		assert.Len(t, o.httpOptions, 2)
	})

	t.Run("zero options is valid", func(t *testing.T) {
		o := newOption()
		opt := WithHTTPOptions()
		opt.apply(o)
		assert.Nil(t, o.httpOptions)
	})
}

func TestOptionComposition(t *testing.T) {
	t.Run("multiple options compose correctly", func(t *testing.T) {
		o := newOption()
		opts := []Option{
			Offset(2048),
			Limit(8192),
			Retry(3),
			WithClient(&http.Client{}),
			WithResponseHeaderTimeout(30 * time.Second),
			WithHTTPOptions(httpc.WithHeaders("X-Foo", "bar")),
		}
		for _, opt := range opts {
			opt.apply(o)
		}
		assert.Equal(t, int64(2048), o.offset)
		assert.Equal(t, int64(8192), o.limit)
		assert.Equal(t, 3, o.retry)
		assert.NotNil(t, o.client)
		assert.Equal(t, 30*time.Second, o.responseHeaderTimeout)
		assert.Len(t, o.httpOptions, 1)
	})
}

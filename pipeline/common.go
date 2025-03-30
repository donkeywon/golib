package pipeline

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"hash"
	"hash/crc32"
	"io"
	"time"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/ratelimit"
	"github.com/donkeywon/golib/runner"
	"github.com/zeebo/xxh3"
)

type closeFunc func() error

type Common interface {
	io.Closer
	runner.Runner
	plugin.Plugin[Type]
	optionApplier
}

type CommonCfg struct {
	Type Type `json:"type" yaml:"type"`
	Cfg  any  `json:"cfg" yaml:"cfg"`
}

type CommonOption struct {
	BufSize             int            `json:"bufSize" yaml:"bufSize"`
	QueueSize           int            `json:"queueSize" yaml:"queueSize"`
	Deadline            int            `json:"deadline" yaml:"deadline"`
	EnableAsync         bool           `json:"enableAsync" yaml:"enableAsync"`
	ProgressLogInterval int            `json:"progressLogInterval" yaml:"progressLogInterval"`
	Hash                string         `json:"hash" yaml:"hash"`
	Checksum            string         `json:"checksum" yaml:"checksum"`
	RateLimitCfg        *ratelimit.Cfg `json:"rateLimitCfg" yaml:"rateLimitCfg"`
}

func (ito *CommonOption) toOptions(write bool) []Option {
	opts := make([]Option, 0, 2)
	if ito.EnableAsync && ito.BufSize > 0 {
		if write {
			opts = append(opts, EnableAsyncWrite(ito.BufSize, ito.QueueSize, time.Second*time.Duration(ito.Deadline)))
		} else {
			opts = append(opts, EnableAsyncRead(ito.BufSize, ito.QueueSize))
		}
	}

	if ito.BufSize > 0 {
		if write {
			opts = append(opts, EnableBufWrite(ito.BufSize))
		} else {
			opts = append(opts, EnableBufRead(ito.BufSize))
		}
	}

	if ito.ProgressLogInterval > 0 {
		opts = append(opts, ProgressLog(time.Second*time.Duration(ito.ProgressLogInterval)))
	}
	if len(ito.Hash) > 0 && len(ito.Checksum) > 0 {
		opts = append(opts, Checksum(ito.Checksum, initHash(ito.Hash)))
	}
	if len(ito.Hash) > 0 {
		opts = append(opts, Hash(initHash(ito.Hash)))
	}
	if ito.RateLimitCfg != nil && ito.RateLimitCfg.Cfg != nil {
		opts = append(opts, RateLimit(ito.RateLimitCfg))
	}

	return opts
}

func initHash(algo string) hash.Hash {
	var h hash.Hash
	switch algo {
	case "sha1":
		h = sha1.New()
	case "md5":
		h = md5.New()
	case "sha256":
		h = sha256.New()
	case "crc32":
		h = crc32.New(crc32.IEEETable)
	case "xxh3":
		h = xxh3.New()
	default:
		h = xxh3.New()
	}
	return h
}

package pipeline

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"hash"
	"hash/crc32"
	"io"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/ratelimit"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/yamls"
	"github.com/tidwall/gjson"
	"github.com/zeebo/xxh3"
)

type closeFunc func() error

type Common interface {
	io.Closer
	runner.Runner
	plugin.Plugin
	optionApplier
}

type CommonCfg struct {
	Type Type `json:"type" yaml:"type"`
	Cfg  any  `json:"cfg" yaml:"cfg"`
}

type commonCfgWithoutType struct {
	Cfg any `json:"cfg" yaml:"cfg"`
}

func (c *CommonCfg) UnmarshalJSON(data []byte) error {
	return c.customUnmarshal(data, jsons.Unmarshal)
}

func (c *CommonCfg) UnmarshalYAML(data []byte) error {
	return c.customUnmarshal(data, yamls.Unmarshal)
}

func (c *CommonCfg) customUnmarshal(data []byte, unmarshaler func([]byte, any) error) error {
	typ := gjson.GetBytes(data, "type")
	if !typ.Exists() {
		return errs.Errorf("type is not present")
	}
	if typ.Type != gjson.String {
		return errs.Errorf("invalid type")
	}
	c.Type = Type(typ.Str)

	cv := commonCfgWithoutType{}
	cv.Cfg = plugin.CreateCfg(c.Type)
	if cv.Cfg == nil {
		return errs.Errorf("unknown type: %s", typ.Str)
	}
	err := unmarshaler(data, &cv)
	if err != nil {
		return err
	}
	c.Cfg = cv.Cfg
	return nil
}

type CommonOption struct {
	BufSize             int            `json:"bufSize" yaml:"bufSize"`
	QueueSize           int            `json:"queueSize" yaml:"queueSize"`
	Deadline            int            `json:"deadline" yaml:"deadline"`
	Async               bool           `json:"async" yaml:"async"`
	ProgressLogInterval int            `json:"progressLogInterval" yaml:"progressLogInterval"`
	Hash                string         `json:"hash" yaml:"hash"`
	Checksum            string         `json:"checksum" yaml:"checksum"`
	RateLimitCfg        *ratelimit.Cfg `json:"rateLimitCfg" yaml:"rateLimitCfg"`
	Count               bool           `json:"count" yaml:"count"`
}

func (ito *CommonOption) toOptions(write bool) []Option {
	if ito == nil {
		return nil
	}
	opts := make([]Option, 0, 2)
	if ito.Async && ito.BufSize > 0 {
		if write {
			opts = append(opts, EnableAsyncWrite(ito.BufSize, ito.QueueSize, time.Second*time.Duration(ito.Deadline)))
		} else {
			opts = append(opts, EnableAsyncRead(ito.BufSize, ito.QueueSize))
		}
	} else if ito.BufSize > 0 {
		if write {
			opts = append(opts, EnableBufWrite(ito.BufSize))
		} else {
			opts = append(opts, EnableBufRead(ito.BufSize))
		}
	}

	if ito.ProgressLogInterval > 0 {
		if write {
			opts = append(opts, ProgressLogWrite(time.Second*time.Duration(ito.ProgressLogInterval)))
		} else {
			opts = append(opts, ProgressLogRead(time.Second*time.Duration(ito.ProgressLogInterval)))
		}
	}
	if len(ito.Hash) > 0 && len(ito.Checksum) > 0 {
		opts = append(opts, Checksum(ito.Checksum, initHash(ito.Hash)))
	} else if len(ito.Hash) > 0 {
		if write {
			opts = append(opts, HashWrite(initHash(ito.Hash)))
		} else {
			opts = append(opts, HashRead(initHash(ito.Hash)))
		}
	}
	if ito.RateLimitCfg != nil {
		if write {
			opts = append(opts, RateLimitWrite(ito.RateLimitCfg))
		} else {
			opts = append(opts, RateLimitRead(ito.RateLimitCfg))
		}
	}
	if ito.Count {
		if write {
			opts = append(opts, CountWrite())
		} else {
			opts = append(opts, CountRead())
		}
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

type CommonCfgWithOption struct {
	*CommonCfg
	*CommonOption
}

func (cc *CommonCfgWithOption) UnmarshalJSON(data []byte) error {
	if cc.CommonCfg == nil {
		cc.CommonCfg = &CommonCfg{}
	}
	err := cc.CommonCfg.UnmarshalJSON(data)
	if err != nil {
		return err
	}
	return cc.customUnmarshal(data, jsons.Unmarshal)
}

func (cc *CommonCfgWithOption) UnmarshalYAML(data []byte) error {
	if cc.CommonCfg == nil {
		cc.CommonCfg = &CommonCfg{}
	}
	err := cc.CommonCfg.UnmarshalYAML(data)
	if err != nil {
		return err
	}
	return cc.customUnmarshal(data, yamls.Unmarshal)
}

func (cc *CommonCfgWithOption) customUnmarshal(data []byte, unmarshaler func([]byte, any) error) error {
	o := &CommonOption{}
	err := unmarshaler(data, o)
	if err != nil {
		return err
	}
	cc.CommonOption = o
	return nil
}

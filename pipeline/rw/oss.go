package rw

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(TypeOss, func() any { return NewOSS() }, func() any { return NewOSSCfg() })
}

const (
	TypeOss Type = "oss"

	defaultOssTimeout = 600
	defaultOssRetry   = 3
)

type OSSCfg struct {
	URL     string `json:"url"      validate:"required" yaml:"url"`
	Ak      string `json:"ak"       validate:"required" yaml:"ak"`
	Sk      string `json:"sk"       validate:"required" yaml:"sk"`
	Region  string `json:"region"   yaml:"region"`
	Append  bool   `json:"append"   yaml:"append"`
	Timeout int    `json:"timeout"  yaml:"timeout"`
	Retry   int    `json:"retry"    yaml:"retry"`
}

func NewOSSCfg() *OSSCfg {
	return &OSSCfg{
		Timeout: defaultOssTimeout,
		Retry:   defaultOssRetry,
	}
}

type OSS struct {
	RW
	*OSSCfg
}

func NewOSS() RW {
	return &OSS{
		RW: CreateBase(string(TypeOss)),
	}
}

func (o *OSS) Init() error {
	if o.IsStarter() {
		return errs.New("oss rw must not be Starter")
	}

	if o.IsReader() {
		r := createOssReader(o.OSSCfg)
		o.NestReader(r)
	} else {
		if o.OSSCfg.Append {
			o.NestWriter(createOssAppendWriter(o.OSSCfg))
		} else {
			o.NestWriter(createOssMultipartWriter(o.OSSCfg))
		}
	}
	o.HookRead(o.hookLogRead)
	o.HookWrite(o.hookLogWrite)

	return o.RW.Init()
}

func (o *OSS) Type() any {
	return TypeOss
}

func (o *OSS) GetCfg() any {
	return o.OSSCfg
}

func (o *OSS) hookLogWrite(n int, bs []byte, err error, cost int64, misc ...any) error {
	o.Info("write", "bs_len", len(bs), "bs_cap", cap(bs), "nw", n, "cost", cost,
		"async_chan_len", o.AsyncChanLen(), "async_chan_cap", o.AsyncChanCap(), "misc", misc, "err", err)
	return nil
}

func (o *OSS) hookLogRead(n int, bs []byte, err error, cost int64, misc ...any) error {
	o.Info("read", "bs_len", len(bs), "bs_cap", cap(bs), "nr", n, "cost", cost,
		"async_chan_len", o.AsyncChanLen(), "async_chan_cap", o.AsyncChanCap(), "misc", misc, "err", err)
	return nil
}

func createOSSCfg(ossCfg *OSSCfg) *oss.Cfg {
	return &oss.Cfg{
		URL:     ossCfg.URL,
		Ak:      ossCfg.Ak,
		Sk:      ossCfg.Sk,
		Retry:   ossCfg.Retry,
		Timeout: ossCfg.Timeout,
		Region:  ossCfg.Region,
	}
}

func createOssReader(ossCfg *OSSCfg) *oss.Reader {
	r := oss.NewReader()
	r.Cfg = createOSSCfg(ossCfg)
	return r
}

func createOssAppendWriter(ossCfg *OSSCfg) *oss.AppendWriter {
	w := oss.NewAppendWriter()
	w.Cfg = createOSSCfg(ossCfg)
	return w
}

func createOssMultipartWriter(ossCfg *OSSCfg) *oss.MultiPartWriter {
	w := oss.NewMultiPartWriter()
	w.Cfg = createOSSCfg(ossCfg)
	return w
}

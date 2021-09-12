package pipeline

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegisterWithCfg(RWTypeOss, func() interface{} { return NewOssRW() }, func() interface{} { return NewOssRWCfg() })
}

const (
	RWTypeOss RWType = "oss"

	defaultOssTimeout = 600
	defaultOssRetry   = 3
)

type OssRWCfg struct {
	URL     string `json:"url"      validate:"required" yaml:"url"`
	Ak      string `json:"ak"       validate:"required" yaml:"ak"`
	Sk      string `json:"sk"       validate:"required" yaml:"sk"`
	Region  string `json:"region"   yaml:"region"`
	Append  bool   `json:"append"   yaml:"append"`
	Timeout int    `json:"timeout"  yaml:"timeout"`
	Retry   int    `json:"retry"    yaml:"retry"`
}

func NewOssRWCfg() *OssRWCfg {
	return &OssRWCfg{
		Timeout: defaultOssTimeout,
		Retry:   defaultOssRetry,
	}
}

type OssRW struct {
	RW
	*OssRWCfg
}

func NewOssRW() RW {
	return &OssRW{
		RW: CreateBaseRW(string(RWTypeOss)),
	}
}

func (o *OssRW) Init() error {
	if o.IsStarter() {
		return errs.New("oss rw must not be Starter")
	}

	if o.IsReader() {
		r := createOssReader(o.OssRWCfg)
		o.NestReader(r)
	} else {
		if o.OssRWCfg.Append {
			o.NestWriter(createOssAppendWriter(o.OssRWCfg))
		} else {
			o.NestWriter(createOssMultipartWriter(o.OssRWCfg))
		}
	}
	o.HookRead(o.hookLogRead)
	o.HookWrite(o.hookLogWrite)

	return o.RW.Init()
}

func (o *OssRW) Type() interface{} {
	return RWTypeOss
}

func (o *OssRW) GetCfg() interface{} {
	return o.OssRWCfg
}

func (o *OssRW) hookLogWrite(n int, bs []byte, err error, cost int64, misc ...interface{}) error {
	o.Info("write", "bs_len", len(bs), "bs_cap", cap(bs), "nw", n, "cost", cost,
		"async_chan_len", o.AsyncChanLen(), "async_chan_cap", o.AsyncChanCap(), "misc", misc, "err", err)
	return nil
}

func (o *OssRW) hookLogRead(n int, bs []byte, err error, cost int64, misc ...interface{}) error {
	o.Info("read", "bs_len", len(bs), "bs_cap", cap(bs), "nr", n, "cost", cost,
		"async_chan_len", o.AsyncChanLen(), "async_chan_cap", o.AsyncChanCap(), "misc", misc, "err", err)
	return nil
}

func createOssCfg(ossCfg *OssRWCfg) *oss.Cfg {
	return &oss.Cfg{
		URL:     ossCfg.URL,
		Ak:      ossCfg.Ak,
		Sk:      ossCfg.Sk,
		Retry:   ossCfg.Retry,
		Timeout: ossCfg.Timeout,
		Region:  ossCfg.Region,
	}
}

func createOssReader(ossCfg *OssRWCfg) *oss.Reader {
	r := oss.NewReader()
	r.Cfg = createOssCfg(ossCfg)
	return r
}

func createOssAppendWriter(ossCfg *OssRWCfg) *oss.AppendWriter {
	w := oss.NewAppendWriter()
	w.Cfg = createOssCfg(ossCfg)
	return w
}

func createOssMultipartWriter(ossCfg *OssRWCfg) *oss.MultiPartWriter {
	w := oss.NewMultiPartWriter()
	w.Cfg = createOssCfg(ossCfg)
	return w
}

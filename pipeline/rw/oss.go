package rw

import (
	"context"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(TypeOSS, func() any { return NewOSS() }, func() any { return NewOSSCfg() })
}

const (
	TypeOSS Type = "oss"

	defaultOSSTimeout = 600
	defaultOSSRetry   = 3
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
		Timeout: defaultOSSTimeout,
		Retry:   defaultOSSRetry,
	}
}

type OSS struct {
	RW
	*OSSCfg
}

func NewOSS() RW {
	return &OSS{
		RW: CreateBase(string(TypeOSS)),
	}
}

func (o *OSS) Init() error {
	if o.IsStarter() {
		return errs.New("oss rw must not be Starter")
	}

	if o.IsReader() {
		r := createOSSReader(o.Ctx(), o.OSSCfg)
		o.NestReader(r)
	} else {
		if o.OSSCfg.Append {
			o.NestWriter(createOSSAppendWriter(o.OSSCfg))
		} else {
			o.NestWriter(createOSSMultipartWriter(o.Ctx(), o.OSSCfg))
		}
	}

	return o.RW.Init()
}

func (o *OSS) Type() any {
	return TypeOSS
}

func (o *OSS) GetCfg() any {
	return o.OSSCfg
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

func createOSSReader(ctx context.Context, ossCfg *OSSCfg) *oss.Reader {
	r := oss.NewReader(ctx, createOSSCfg(ossCfg))
	return r
}

func createOSSAppendWriter(ossCfg *OSSCfg) *oss.AppendWriter {
	w := oss.NewAppendWriter()
	w.Cfg = createOSSCfg(ossCfg)
	return w
}

func createOSSMultipartWriter(ctx context.Context, ossCfg *OSSCfg) *oss.MultiPartWriter {
	w := oss.NewMultiPartWriter(ctx, createOSSCfg(ossCfg))
	return w
}

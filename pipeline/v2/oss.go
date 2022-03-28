package v2

import (
	"context"
	"io"

	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(ReaderOSS, func() *OSS { return NewOSS(ReaderOSS) }, NewOSSCfg)
	plugin.RegWithCfg(WriterOSS, func() *OSS { return NewOSS(WriterOSS) }, NewOSSCfg)
}

const (
	ReaderOSS Type = "ross"
	WriterOSS Type = "woss"
)

type OSSCfg struct {
	*oss.Cfg
	Append bool `json:"append" yaml:"append"`
}

func NewOSSCfg() *OSSCfg {
	return &OSSCfg{
		Cfg: &oss.Cfg{},
	}
}

type OSS struct {
	Common
	Reader
	Writer

	typ Type
	c   *OSSCfg
}

func NewOSS(typ Type) *OSS {
	o := &OSS{
		typ: typ,
		c:   NewOSSCfg(),
	}

	if o.typ == ReaderOSS {
		r := CreateReader(string(typ))
		o.Common = r
		o.Reader = r
	} else {
		w := CreateWriter(string(typ))
		o.Common = w
		o.Writer = w
	}

	return o
}

func (o *OSS) Init() error {
	if o.typ == ReaderOSS {
		o.Common.(Reader).WrapReader(createOSSReader(o.Ctx(), o.c))
	} else {
		if o.c.Append {
			o.Common.(Writer).WrapWriter(createOSSAppendWriter(o.Ctx(), o.c))
		} else {
			o.Common.(Writer).WrapWriter(createOSSMultipartWriter(o.Ctx(), o.c))
		}
	}

	return o.Common.Init()
}

func (o *OSS) WrapReader(io.ReadCloser) {
	panic(ErrInvalidWrap)
}

func (o *OSS) WrapWriter(io.WriteCloser) {
	panic(ErrInvalidWrap)
}

func (o *OSS) Type() Type {
	return o.typ
}

func (o *OSS) SetCfg(cfg *OSSCfg) {
	o.c = cfg
}

func createOSSReader(ctx context.Context, cfg *OSSCfg) *oss.Reader {
	r := oss.NewReader(ctx, cfg.Cfg)
	return r
}

func createOSSAppendWriter(ctx context.Context, cfg *OSSCfg) *oss.AppendWriter {
	w := oss.NewAppendWriter(ctx, cfg.Cfg)
	return w
}

func createOSSMultipartWriter(ctx context.Context, cfg *OSSCfg) *oss.MultiPartWriter {
	w := oss.NewMultiPartWriter(ctx, cfg.Cfg)
	return w
}

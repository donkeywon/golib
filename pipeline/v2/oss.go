package v2

import (
	"context"
	"io"

	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(ReaderOSS, func() Common { return NewOSSReader() }, func() any { return NewOSSCfg() })
	plugin.RegWithCfg(WriterOSS, func() Common { return NewOSSWriter() }, func() any { return NewOSSCfg() })
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

type OSSReader struct {
	Reader
	*OSSCfg
}

func NewOSSReader() *OSSReader {
	return &OSSReader{
		Reader: CreateReader(string(ReaderOSS)),
		OSSCfg: NewOSSCfg(),
	}
}

func (o *OSSReader) Init() error {
	o.Reader.WrapReader(createOSSReader(o.Ctx(), o.OSSCfg))
	return o.Reader.Init()
}

func (o *OSSReader) Wrap(io.ReadCloser) {
	panic(ErrInvalidWrap)
}

func (o *OSSReader) Type() Type {
	return ReaderOSS
}

func (o *OSSReader) SetCfg(c any) {
	o.OSSCfg = c.(*OSSCfg)
}

type OSSWriter struct {
	Writer
	*OSSCfg
}

func NewOSSWriter() *OSSWriter {
	return &OSSWriter{
		Writer: CreateWriter(string(WriterOSS)),
		OSSCfg: NewOSSCfg(),
	}
}

func (o *OSSWriter) Init() error {
	if o.OSSCfg.Append {
		o.Writer.WrapWriter(createOSSAppendWriter(o.Ctx(), o.OSSCfg))
	} else {
		o.Writer.WrapWriter(createOSSMultipartWriter(o.Ctx(), o.OSSCfg))
	}
	return o.Writer.Init()
}

func (o *OSSWriter) Wrap(io.WriteCloser) {
	panic(ErrInvalidWrap)
}

func (o *OSSWriter) Type() Type {
	return WriterOSS
}

func (o *OSSWriter) SetCfg(c any) {
	o.OSSCfg = c.(*OSSCfg)
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

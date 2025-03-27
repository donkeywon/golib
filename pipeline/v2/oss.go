package v2

import (
	"context"
	"io"

	"github.com/donkeywon/golib/oss"
)

const (
	TypeOSSReader ReaderType = "oss"
	TypeOSSWriter WriterType = "oss"
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
		Reader: CreateReader(string(TypeOSSReader)),
		OSSCfg: NewOSSCfg(),
	}
}

func (o *OSSReader) Init() error {
	o.Reader.Wrap(createOSSReader(o.Ctx(), o.OSSCfg))
	return o.Reader.Init()
}

func (o *OSSReader) Wrap(io.ReadCloser) {
	panic(ErrInvalidWrap)
}

func (o *OSSReader) Type() any {
	return TypeOSSReader
}

func (o *OSSReader) GetCfg() any {
	return o.Cfg
}

type OSSWriter struct {
	Writer
	*OSSCfg
}

func NewOSSWriter() *OSSWriter {
	return &OSSWriter{
		Writer: CreateWriter(string(TypeOSSWriter)),
		OSSCfg: NewOSSCfg(),
	}
}

func (o *OSSWriter) Init() error {
	if o.OSSCfg.Append {
		o.Writer.Wrap(createOSSAppendWriter(o.Ctx(), o.OSSCfg))
	} else {
		o.Writer.Wrap(createOSSMultipartWriter(o.Ctx(), o.OSSCfg))
	}
	return o.Writer.Init()
}

func (o *OSSWriter) Wrap(io.WriteCloser) {
	panic(ErrInvalidWrap)
}

func (o *OSSWriter) Type() any {
	return TypeOSSWriter
}

func (o *OSSWriter) GetCfg() any {
	return o.OSSCfg
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

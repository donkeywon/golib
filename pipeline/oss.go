package pipeline

import (
	"context"
	"io"

	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(ReaderOSS, func() Reader { return NewOSSReader() }, func() any { return NewOSSCfg() })
	plugin.RegWithCfg(WriterOSS, func() Writer { return NewOSSWriter() }, func() any { return NewOSSCfg() })
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

	r *oss.Reader
}

func NewOSSReader() *OSSReader {
	return &OSSReader{
		Reader: CreateReader(string(ReaderOSS)),
		OSSCfg: NewOSSCfg(),
	}
}

func (o *OSSReader) Init() error {
	o.r = createOSSReader(o.Ctx(), o.OSSCfg)
	o.Reader.WrapReader(o.r)
	return o.Reader.Init()
}

func (o *OSSReader) WrapReader(io.Reader) {
	panic(ErrInvalidWrap)
}

func (o *OSSReader) SetCfg(c any) {
	o.OSSCfg = c.(*OSSCfg)
}

func (o *OSSReader) Size() int64 {
	return o.r.Size()
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
		mw := createOSSMultipartWriter(o.Ctx(), o.OSSCfg)
		mw.OnUploadPart(func(uploadWorker int, partNo int, partSize int, etag string, err error) {
			o.Debug("upload part", "upload_worker", uploadWorker, "part_no", partNo, "part_size", partSize, "etag", etag, "err", err)
		})
		mw.OnComplete(func(uploadID string, body string, err error) {
			o.Debug("complete upload", "upload_id", uploadID, "body", body, "err", err)
		})
		o.Writer.WrapWriter(mw)
	}
	return o.Writer.Init()
}

func (o *OSSWriter) WrapWriter(io.Writer) {
	panic(ErrInvalidWrap)
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

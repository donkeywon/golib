package pipeline

import (
	"io"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/rwwrapper"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	"github.com/klauspost/pgzip"
)

func init() {
	plugin.Register(RWTypeCompress, func() interface{} { return NewCompressRW() })
	plugin.RegisterCfg(RWTypeCompress, func() interface{} { return NewCompressRWCfg() })
}

const (
	RWTypeCompress RWType = "compress"

	CompressTypeNop    CompressType = "nop"
	CompressTypeGzip   CompressType = "gzip"
	CompressTypeSnappy CompressType = "snappy"
	CompressTypeZstd   CompressType = "zstd"

	CompressLevelFast   CompressLevel = "fast"
	CompressLevelBetter CompressLevel = "better"
	CompressLevelBest   CompressLevel = "best"

	gzipDefaultBlockSize = 1024 * 1024
)

type CompressType string
type CompressLevel string

type CompressRWCfg struct {
	Type        CompressType  `json:"type"        validate:"required" yaml:"type"`
	Level       CompressLevel `json:"level"       validate:"required" yaml:"level"`
	Concurrency int           `json:"concurrency" yaml:"concurrency"`
}

func NewCompressRWCfg() *CompressRWCfg {
	return &CompressRWCfg{}
}

type CompressRW struct {
	RW
	*CompressRWCfg
}

func NewCompressRW() *CompressRW {
	return &CompressRW{
		RW: NewBaseRW(string(RWTypeCompress)),
	}
}

func (c *CompressRW) Init() error {
	c.RW.WithLoggerNoName(c.Logger(), "type", c.CompressRWCfg.Type)
	return c.RW.Init()
}

func (c *CompressRW) NestReader(r io.ReadCloser) error {
	switch c.CompressRWCfg.Type {
	case CompressTypeNop:
		return c.RW.NestReader(r)
	case CompressTypeGzip:
		return c.RW.NestReader(NewGzipReader(r, c.CompressRWCfg))
	case CompressTypeSnappy:
		return c.RW.NestReader(NewSnappyReader(r, c.CompressRWCfg))
	case CompressTypeZstd:
		return c.RW.NestReader(NewZstdReader(r, c.CompressRWCfg))
	default:
		return c.RW.NestReader(r)
	}
}

func (c *CompressRW) NestWriter(w io.WriteCloser) error {
	switch c.CompressRWCfg.Type {
	case CompressTypeNop:
		return c.RW.NestWriter(w)
	case CompressTypeGzip:
		return c.RW.NestWriter(NewGzipWritter(w, c.CompressRWCfg))
	case CompressTypeSnappy:
		return c.RW.NestWriter(NewSnappyWriter(w, c.CompressRWCfg))
	case CompressTypeZstd:
		return c.RW.NestWriter(NewZstdWriter(w, c.CompressRWCfg))
	default:
		return c.RW.NestWriter(w)
	}
}

func (c *CompressRW) Nwrite() uint64 {
	if c.Writer() == nil {
		return 0
	}

	type writer interface {
		Nwrite() uint64
	}

	if w, ok := c.Writer().(writer); ok {
		return w.Nwrite()
	}
	return c.RW.Nwrite()
}

func (c *CompressRW) Type() interface{} {
	return RWTypeCompress
}

func (c *CompressRW) GetCfg() interface{} {
	return c.CompressRWCfg
}

func NewZstdWriter(w io.WriteCloser, cfg *CompressRWCfg) *rwwrapper.WriterWrapperr {
	ww := rwwrapper.NewWriterWrapperr()
	iw := rwwrapper.NewWriterWrapper(w)
	var ze *zstd.Encoder
	opts := make([]zstd.EOption, 0)
	if cfg.Concurrency > 0 {
		opts = append(opts, zstd.WithEncoderConcurrency(cfg.Concurrency))
	}
	switch cfg.Level {
	case CompressLevelFast:
		opts = append(opts, zstd.WithEncoderLevel(zstd.SpeedFastest))
	case CompressLevelBetter:
		opts = append(opts, zstd.WithEncoderLevel(zstd.SpeedDefault))
	case CompressLevelBest:
		opts = append(opts, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	default:
		opts = append(opts, zstd.WithEncoderLevel(zstd.SpeedDefault))
	}
	ze, _ = zstd.NewWriter(iw, opts...)
	ww.Set(ze, iw)
	return ww
}

func NewSnappyWriter(w io.WriteCloser, cfg *CompressRWCfg) *rwwrapper.WriterWrapperr {
	ww := rwwrapper.NewWriterWrapperr()
	iw := rwwrapper.NewWriterWrapper(w)
	var sw *s2.Writer
	opts := make([]s2.WriterOption, 0)
	opts = append(opts, s2.WriterSnappyCompat())
	if cfg.Concurrency > 0 {
		opts = append(opts, s2.WriterConcurrency(cfg.Concurrency))
	}
	switch cfg.Level {
	case CompressLevelFast:
	case CompressLevelBetter:
		opts = append(opts, s2.WriterBetterCompression())
	case CompressLevelBest:
		opts = append(opts, s2.WriterBestCompression())
	default:
		opts = append(opts, s2.WriterBetterCompression())
	}
	sw = s2.NewWriter(iw, opts...)
	ww.Set(sw, iw)
	return ww
}

func NewGzipWritter(w io.WriteCloser, cfg *CompressRWCfg) *rwwrapper.WriterWrapperr {
	ww := rwwrapper.NewWriterWrapperr()
	iw := rwwrapper.NewWriterWrapper(w)
	var gw *pgzip.Writer
	switch cfg.Level {
	case CompressLevelFast:
		gw, _ = pgzip.NewWriterLevel(iw, gzip.BestSpeed)
	case CompressLevelBetter:
		gw, _ = pgzip.NewWriterLevel(iw, gzip.DefaultCompression)
	case CompressLevelBest:
		gw, _ = pgzip.NewWriterLevel(iw, gzip.BestCompression)
	default:
		gw, _ = pgzip.NewWriterLevel(iw, gzip.DefaultCompression)
	}
	if cfg.Concurrency > 0 {
		_ = gw.SetConcurrency(gzipDefaultBlockSize, cfg.Concurrency)
	}
	ww.Set(gw, iw)
	return ww
}

func NewZstdReader(r io.ReadCloser, cfg *CompressRWCfg) *rwwrapper.ReaderWrapperr {
	ir := rwwrapper.NewReaderWrapper(r)
	var zr *zstd.Decoder
	if cfg.Concurrency > 0 {
		zr, _ = zstd.NewReader(ir, zstd.WithDecoderConcurrency(cfg.Concurrency))
	} else {
		zr, _ = zstd.NewReader(ir)
	}

	rw := rwwrapper.NewReaderWrapperr()
	rw.Set(zr, ir)
	return rw
}

func NewSnappyReader(r io.ReadCloser, _ *CompressRWCfg) *rwwrapper.ReaderWrapperr {
	ir := rwwrapper.NewReaderWrapper(r)
	sr := s2.NewReader(ir)

	rw := rwwrapper.NewReaderWrapperr()
	rw.Set(sr, ir)
	return rw
}

func NewGzipReader(r io.ReadCloser, cfg *CompressRWCfg) *rwwrapper.ReaderWrapperr {
	ir := rwwrapper.NewReaderWrapper(r)
	var gr *pgzip.Reader
	if cfg.Concurrency > 0 {
		gr, _ = pgzip.NewReaderN(ir, gzipDefaultBlockSize, cfg.Concurrency)
	} else {
		gr, _ = pgzip.NewReader(ir)
	}

	rw := rwwrapper.NewReaderWrapperr()
	rw.Set(gr, ir)
	return rw
}

func FileExtFromCompressCfg(c *CompressRWCfg) string {
	switch c.Type {
	case CompressTypeNop:
		return ""
	case CompressTypeGzip:
		return ".gz"
	case CompressTypeSnappy:
		return ".snappy"
	case CompressTypeZstd:
		return ".zst"
	default:
		return ""
	}
}

package v2

import (
	"errors"
	"io"

	"github.com/donkeywon/golib/plugin"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	"github.com/klauspost/pgzip"
)

func init() {
	plugin.RegWithCfg(ReaderCompress, func() *Compress { return NewCompress(ReaderCompress) }, NewCompressCfg)
	plugin.RegWithCfg(WriterCompress, func() *Compress { return NewCompress(WriterCompress) }, NewCompressCfg)
}

const (
	ReaderCompress Type = "rcompress"
	WriterCompress Type = "wcompress"

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

type CompressCfg struct {
	Type        CompressType  `json:"type"        validate:"required" yaml:"type"`
	Level       CompressLevel `json:"level"       validate:"required" yaml:"level"`
	Concurrency int           `json:"concurrency" yaml:"concurrency"`
}

func NewCompressCfg() *CompressCfg {
	return &CompressCfg{}
}

type Compress struct {
	Common
	Reader
	Writer

	c   *CompressCfg
	typ Type
	r   io.ReadCloser
	w   io.WriteCloser
}

func NewCompress(typ Type) *Compress {
	c := &Compress{
		c:   NewCompressCfg(),
		typ: typ,
	}

	if c.typ == ReaderCompress {
		r := CreateReader(string(typ))
		c.Common = r
		c.Reader = r
	} else {
		w := CreateWriter(string(typ))
		c.Common = w
		c.Writer = w
	}

	return c
}

func (c *Compress) Init() error {
	c.WithLoggerFields("type", c.c.Type)
	return c.Common.Init()
}

func (c *Compress) Type() Type {
	return c.typ
}

func (c *Compress) SetCfg(cfg *CompressCfg) {
	c.c = cfg
}

func (c *Compress) WrapReader(r io.ReadCloser) {
	var compressReader io.ReadCloser
	switch c.c.Type {
	case CompressTypeGzip:
		compressReader = NewGzipReader(r, c.c)
	case CompressTypeSnappy:
		compressReader = NewSnappyReader(r, c.c)
	case CompressTypeZstd:
		compressReader = NewZstdReader(r, c.c)
	default:
	}

	if compressReader == nil {
		c.Common.(Reader).WrapReader(r)
	} else {
		c.Common.(Reader).WrapReader(compressReader)
		c.r = r
	}
}

func (c *Compress) WrapWriter(w io.WriteCloser) {
	var compressWriter io.WriteCloser
	switch c.c.Type {
	case CompressTypeGzip:
		compressWriter = NewGzipWritter(w, c.c)
	case CompressTypeSnappy:
		compressWriter = NewSnappyWriter(w, c.c)
	case CompressTypeZstd:
		compressWriter = NewZstdWriter(w, c.c)
	default:
	}

	if compressWriter == nil {
		c.Common.(Writer).WrapWriter(w)
	} else {
		c.Common.(Writer).WrapWriter(compressWriter)
		c.w = w
	}
}

func (c *Compress) Close() error {
	if c.typ == ReaderCompress {
		if c.r != nil {
			return errors.Join(c.Common.Close(), c.r.Close())
		}
		return c.Common.Close()
	} else {
		if c.w != nil {
			return errors.Join(c.Common.Close(), c.w.Close())
		}
		return c.Common.Close()
	}
}

func NewZstdWriter(w io.WriteCloser, cfg *CompressCfg) io.WriteCloser {
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
	ze, _ := zstd.NewWriter(w, opts...)
	return ze
}

func NewSnappyWriter(w io.WriteCloser, cfg *CompressCfg) io.WriteCloser {
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
	sw := s2.NewWriter(w, opts...)
	return sw
}

func NewGzipWritter(w io.WriteCloser, cfg *CompressCfg) io.WriteCloser {
	var gw *pgzip.Writer
	switch cfg.Level {
	case CompressLevelFast:
		gw, _ = pgzip.NewWriterLevel(w, pgzip.BestSpeed)
	case CompressLevelBetter:
		gw, _ = pgzip.NewWriterLevel(w, pgzip.DefaultCompression)
	case CompressLevelBest:
		gw, _ = pgzip.NewWriterLevel(w, pgzip.BestCompression)
	default:
		gw, _ = pgzip.NewWriterLevel(w, pgzip.DefaultCompression)
	}
	if cfg.Concurrency > 0 {
		_ = gw.SetConcurrency(gzipDefaultBlockSize, cfg.Concurrency)
	}
	return gw
}

type zstdReader struct {
	*zstd.Decoder
}

func (z *zstdReader) Close() error {
	z.Decoder.Close()
	return nil
}

func NewZstdReader(r io.ReadCloser, cfg *CompressCfg) io.ReadCloser {
	var zr *zstd.Decoder
	if cfg.Concurrency > 0 {
		zr, _ = zstd.NewReader(r, zstd.WithDecoderConcurrency(cfg.Concurrency))
	} else {
		zr, _ = zstd.NewReader(r)
	}

	return &zstdReader{Decoder: zr}
}

type s2Reader struct {
	*s2.Reader
}

func (s *s2Reader) Close() error {
	return nil
}

func NewSnappyReader(r io.ReadCloser, _ *CompressCfg) io.ReadCloser {
	return &s2Reader{Reader: s2.NewReader(r)}
}

func NewGzipReader(r io.ReadCloser, cfg *CompressCfg) io.ReadCloser {
	var gr *pgzip.Reader
	if cfg.Concurrency > 0 {
		gr, _ = pgzip.NewReaderN(r, gzipDefaultBlockSize, cfg.Concurrency)
	} else {
		gr, _ = pgzip.NewReader(r)
	}
	return gr
}

func FileExtFromCompressCfg(c *CompressCfg) string {
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

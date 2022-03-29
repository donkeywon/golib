package rw

import (
	"errors"
	"io"

	"github.com/donkeywon/golib/plugin"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	"github.com/klauspost/pgzip"
)

func init() {
	plugin.RegWithCfg(TypeCompress, func() RW { return NewCompress() }, func() any { return NewCompressCfg() })
}

const (
	TypeCompress Type = "compress"

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
	RW
	*CompressCfg

	compressReader io.ReadCloser
	compressWriter io.WriteCloser
	reader         io.ReadCloser
	writer         io.WriteCloser
}

func NewCompress() *Compress {
	return &Compress{
		RW: CreateBase(string(TypeCompress)),
	}
}

func (c *Compress) Init() error {
	c.RW.WithLoggerFields("type", c.CompressCfg.Type)
	return c.RW.Init()
}

func (c *Compress) Close() error {
	if c.CompressCfg.Type == CompressTypeNop {
		return c.RW.Close()
	}
	if c.IsReader() {
		return errors.Join(c.RW.Close(), c.reader.Close())
	} else {
		return errors.Join(c.RW.Close(), c.writer.Close())
	}
}

func (c *Compress) NestReader(r io.ReadCloser) {
	c.reader = r
	switch c.CompressCfg.Type {
	case CompressTypeNop:
		c.compressReader = r
	case CompressTypeGzip:
		c.compressReader = NewGzipReader(r, c.CompressCfg)
	case CompressTypeSnappy:
		c.compressReader = NewSnappyReader(r, c.CompressCfg)
	case CompressTypeZstd:
		c.compressReader = NewZstdReader(r, c.CompressCfg)
	default:
		c.compressReader = r
	}

	c.RW.NestReader(c.compressReader)
}

func (c *Compress) NestWriter(w io.WriteCloser) {
	c.writer = w
	switch c.CompressCfg.Type {
	case CompressTypeNop:
		c.compressWriter = w
	case CompressTypeGzip:
		c.compressWriter = NewGzipWritter(w, c.CompressCfg)
	case CompressTypeSnappy:
		c.compressWriter = NewSnappyWriter(w, c.CompressCfg)
	case CompressTypeZstd:
		c.compressWriter = NewZstdWriter(w, c.CompressCfg)
	default:
		c.compressWriter = w
	}

	c.RW.NestWriter(c.compressWriter)
}

func (c *Compress) Type() Type {
	return TypeCompress
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
		gw, _ = pgzip.NewWriterLevel(w, gzip.BestSpeed)
	case CompressLevelBetter:
		gw, _ = pgzip.NewWriterLevel(w, gzip.DefaultCompression)
	case CompressLevelBest:
		gw, _ = pgzip.NewWriterLevel(w, gzip.BestCompression)
	default:
		gw, _ = pgzip.NewWriterLevel(w, gzip.DefaultCompression)
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

package pipeline

import (
	"io"

	"github.com/donkeywon/golib/plugin"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	"github.com/klauspost/pgzip"
)

func init() {
	plugin.RegWithCfg(ReaderCompress, func() Reader { return NewCompressReader() }, func() any { return NewCompressCfg() })
	plugin.RegWithCfg(WriterCompress, func() Writer { return NewCompressWriter() }, func() any { return NewCompressCfg() })
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

type CompressReader struct {
	Reader
	*CompressCfg
}

func NewCompressReader() *CompressReader {
	return &CompressReader{
		Reader:      CreateReader(string(ReaderCompress)),
		CompressCfg: NewCompressCfg(),
	}
}

func (c *CompressReader) Init() error {
	c.WithLoggerFields("type", c.CompressCfg.Type)
	return c.Reader.Init()
}

func (c *CompressReader) WrapReader(r io.Reader) {
	var compressReader io.ReadCloser
	switch c.CompressCfg.Type {
	case CompressTypeGzip:
		compressReader = NewGzipReader(r, c.CompressCfg)
	case CompressTypeSnappy:
		compressReader = NewSnappyReader(r, c.CompressCfg)
	case CompressTypeZstd:
		compressReader = NewZstdReader(r, c.CompressCfg)
	default:
	}

	if compressReader == nil {
		c.Reader.WrapReader(r)
	} else {
		c.Reader.WrapReader(compressReader)
	}
}

func (c *CompressReader) SetCfg(cfg any) {
	c.CompressCfg = cfg.(*CompressCfg)
}

type CompressWriter struct {
	Writer
	*CompressCfg
}

func NewCompressWriter() *CompressWriter {
	return &CompressWriter{
		Writer:      CreateWriter(string(WriterCompress)),
		CompressCfg: NewCompressCfg(),
	}
}

func (c *CompressWriter) Init() error {
	c.WithLoggerFields("type", c.CompressCfg.Type)
	return c.Writer.Init()
}

func (c *CompressWriter) WrapWriter(w io.Writer) {
	var compressWriter io.WriteCloser
	switch c.CompressCfg.Type {
	case CompressTypeGzip:
		compressWriter = NewGzipWritter(w, c.CompressCfg)
	case CompressTypeSnappy:
		compressWriter = NewSnappyWriter(w, c.CompressCfg)
	case CompressTypeZstd:
		compressWriter = NewZstdWriter(w, c.CompressCfg)
	default:
	}

	if compressWriter == nil {
		c.Writer.WrapWriter(w)
	} else {
		c.Writer.WrapWriter(compressWriter)
	}
}

func (c *CompressWriter) SetCfg(cfg any) {
	c.CompressCfg = cfg.(*CompressCfg)
}

func NewZstdWriter(w io.Writer, cfg *CompressCfg) io.WriteCloser {
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

func NewSnappyWriter(w io.Writer, cfg *CompressCfg) io.WriteCloser {
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

func NewGzipWritter(w io.Writer, cfg *CompressCfg) io.WriteCloser {
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

func NewZstdReader(r io.Reader, cfg *CompressCfg) io.ReadCloser {
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

func NewSnappyReader(r io.Reader, _ *CompressCfg) io.ReadCloser {
	return &s2Reader{Reader: s2.NewReader(r)}
}

func NewGzipReader(r io.Reader, cfg *CompressCfg) io.ReadCloser {
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

package pipeline

import (
	"io"
	"strconv"
	"strings"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	"github.com/klauspost/pgzip"
	"github.com/pierrec/lz4/v4"
)

func init() {
	plugin.Reg(ReaderCompress, func() Reader { return NewCompressReader() }, func() any { return NewCompressCfg() })
	plugin.Reg(WriterCompress, func() Writer { return NewCompressWriter() }, func() any { return NewCompressCfg() })
}

const (
	ReaderCompress Type = "rcompress"
	WriterCompress Type = "wcompress"

	CompressTypeNop    CompressType = "nop"
	CompressTypeGzip   CompressType = "gzip"
	CompressTypeSnappy CompressType = "snappy"
	CompressTypeZstd   CompressType = "zstd"
	CompressTypeLz4    CompressType = "lz4"

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
	Concurrency int           `json:"concurrency"                     yaml:"concurrency"`
}

func NewCompressCfg() *CompressCfg {
	return &CompressCfg{}
}

func (c *CompressCfg) String() string {
	var sb strings.Builder
	sb.WriteString(`{"type":"`)
	sb.WriteString(string(c.Type))
	sb.WriteString(`","level":"`)
	sb.WriteString(string(c.Level))
	sb.WriteString(`","concurrency":`)
	sb.WriteString(strconv.Itoa(c.Concurrency))
	sb.WriteString("}")
	return sb.String()
}

type CompressReader struct {
	Reader
	*CompressCfg

	r io.Reader
}

func NewCompressReader() *CompressReader {
	return &CompressReader{
		Reader:      CreateReader(string(ReaderCompress)),
		CompressCfg: NewCompressCfg(),
	}
}

func (c *CompressReader) Init() error {
	var compressReader io.ReadCloser
	switch c.CompressCfg.Type {
	case CompressTypeGzip:
		compressReader = NewGzipReader(c.r, c.CompressCfg)
	case CompressTypeSnappy:
		compressReader = NewSnappyReader(c.r, c.CompressCfg)
	case CompressTypeZstd:
		compressReader = NewZstdReader(c.r, c.CompressCfg)
	case CompressTypeLz4:
		compressReader = NewLz4Reader(c.r, c.CompressCfg)
	default:
	}

	if compressReader == nil {
		c.Reader.WrapReader(c.r)
	} else {
		c.Reader.WrapReader(compressReader)
	}

	c.WithLoggerFields("type", c.CompressCfg.Type)
	return c.Reader.Init()
}

func (c *CompressReader) WrapReader(r io.Reader) {
	c.r = r
}

func (c *CompressReader) SetCfg(cfg any) {
	c.CompressCfg = cfg.(*CompressCfg)
}

type CompressWriter struct {
	Writer
	*CompressCfg

	w io.Writer
}

func NewCompressWriter() *CompressWriter {
	return &CompressWriter{
		Writer:      CreateWriter(string(WriterCompress)),
		CompressCfg: NewCompressCfg(),
	}
}

func (c *CompressWriter) Init() error {
	var compressWriter io.WriteCloser
	switch c.CompressCfg.Type {
	case CompressTypeGzip:
		compressWriter = NewGzipWriter(c.w, c.CompressCfg)
	case CompressTypeSnappy:
		compressWriter = NewSnappyWriter(c.w, c.CompressCfg)
	case CompressTypeZstd:
		compressWriter = NewZstdWriter(c.w, c.CompressCfg)
	case CompressTypeLz4:
		compressWriter = NewLz4Writer(c.w, c.CompressCfg)
	default:
	}

	if compressWriter == nil {
		c.Writer.WrapWriter(c.w)
	} else {
		c.Writer.WrapWriter(compressWriter)
	}

	c.WithLoggerFields("type", c.CompressCfg.Type)
	return c.Writer.Init()
}

func (c *CompressWriter) WrapWriter(w io.Writer) {
	c.w = w
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
	ze, err := zstd.NewWriter(w, opts...)
	if err != nil {
		panic(errs.Wrapf(err, "create zstd writer failed: %s", cfg.String()))
	}
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

func NewGzipWriter(w io.Writer, cfg *CompressCfg) io.WriteCloser {
	var (
		gw  *pgzip.Writer
		err error
	)
	switch cfg.Level {
	case CompressLevelFast:
		gw, err = pgzip.NewWriterLevel(w, pgzip.BestSpeed)
	case CompressLevelBetter:
		gw, err = pgzip.NewWriterLevel(w, pgzip.DefaultCompression)
	case CompressLevelBest:
		gw, err = pgzip.NewWriterLevel(w, pgzip.BestCompression)
	default:
		gw, err = pgzip.NewWriterLevel(w, pgzip.DefaultCompression)
	}
	if err != nil {
		panic(errs.Wrapf(err, "create gzip writer failed: %s", cfg.String()))
	}
	if cfg.Concurrency > 0 {
		err = gw.SetConcurrency(gzipDefaultBlockSize, cfg.Concurrency)
		if err != nil {
			panic(errs.Wrapf(err, "set gzip concurrency failed: %s", cfg.String()))
		}
	}
	return gw
}

func NewLz4Writer(w io.Writer, cfg *CompressCfg) io.WriteCloser {
	opts := make([]lz4.Option, 0)
	if cfg.Concurrency > 0 {
		opts = append(opts, lz4.ConcurrencyOption(cfg.Concurrency))
	}
	switch cfg.Level {
	case CompressLevelFast:
		opts = append(opts, lz4.CompressionLevelOption(lz4.Fast))
	case CompressLevelBetter:
		opts = append(opts, lz4.CompressionLevelOption(lz4.Level1))
	case CompressLevelBest:
		opts = append(opts, lz4.CompressionLevelOption(lz4.Level9))
	default:
		opts = append(opts, lz4.CompressionLevelOption(lz4.Fast))
	}

	lw := lz4.NewWriter(w)
	err := lw.Apply(opts...)
	if err != nil {
		panic(errs.Wrapf(err, "apply lz4 writer options failed: %s", cfg.String()))
	}
	return lw
}

type zstdReader struct {
	*zstd.Decoder
}

func (z *zstdReader) Close() error {
	z.Decoder.Close()
	return nil
}

func NewZstdReader(r io.Reader, cfg *CompressCfg) io.ReadCloser {
	var (
		zr  *zstd.Decoder
		err error
	)
	if cfg.Concurrency > 0 {
		zr, err = zstd.NewReader(r, zstd.WithDecoderConcurrency(cfg.Concurrency))
	} else {
		zr, err = zstd.NewReader(r)
	}
	if err != nil {
		panic(errs.Wrapf(err, "create zstd reader failed: %s", cfg.String()))
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
	var (
		gr  *pgzip.Reader
		err error
	)
	if cfg.Concurrency > 0 {
		gr, err = pgzip.NewReaderN(r, gzipDefaultBlockSize, cfg.Concurrency)
	} else {
		gr, err = pgzip.NewReader(r)
	}
	if err != nil {
		panic(errs.Wrapf(err, "create gzip reader failed: %s", cfg.String()))
	}
	return gr
}

type lz4Reader struct {
	*lz4.Reader
}

func (l *lz4Reader) Close() error {
	return nil
}

func NewLz4Reader(r io.Reader, cfg *CompressCfg) io.ReadCloser {
	lr := lz4.NewReader(r)
	if cfg.Concurrency > 0 {
		err := lr.Apply(lz4.ConcurrencyOption(cfg.Concurrency))
		if err != nil {
			panic(errs.Wrapf(err, "set lz4 reader concurrency option failed: %s", cfg.String()))
		}
	}
	return &lz4Reader{Reader: lr}
}

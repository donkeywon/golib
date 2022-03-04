package encoder

import (
	"github.com/donkeywon/golib/util/bufferpool"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

const (
	liteTimeEncoderLayout = "0102 15:04:05.000000"
)

type consoleLiteEncoder struct {
	zapcore.Encoder

	cfg zapcore.EncoderConfig
}

func NewConsoleLiteEncoder(cfg zapcore.EncoderConfig) (zapcore.Encoder, error) {
	cfg.EncodeLevel = nil
	cfg.EncodeTime = nil
	cfg.EncodeCaller = nil
	cfg.EncodeName = zapcore.FullNameEncoder
	return &consoleLiteEncoder{
		Encoder: zapcore.NewConsoleEncoder(cfg),
		cfg:     cfg,
	}, nil
}

func (c consoleLiteEncoder) Clone() zapcore.Encoder {
	return consoleLiteEncoder{
		Encoder: c.Encoder.Clone(),
		cfg:     c.cfg,
	}
}

func (c consoleLiteEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	buf := bufferpool.Get()
	buf.WriteByte(ent.Level.CapitalString()[0])
	if !ent.Time.IsZero() {
		t := ent.Time.AppendFormat(buf.AvailableBuffer(), liteTimeEncoderLayout)
		buf.Write(t)
	}
	buf.WriteString(c.cfg.ConsoleSeparator)
	buf.WriteString(ent.LoggerName)
	if ent.Caller.Defined {
		if ent.LoggerName != "" {
			buf.WriteByte('(')
		}
		buf.WriteString(ent.Caller.TrimmedPath())
		if ent.LoggerName != "" {
			buf.WriteByte(')')
		}
	}
	ent.LoggerName = buf.String()
	buf.Free()

	return c.Encoder.EncodeEntry(ent, fields)
}

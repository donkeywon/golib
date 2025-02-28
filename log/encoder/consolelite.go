package encoder

import (
	"bytes"

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
	}
}

func (c consoleLiteEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	lite := bytes.NewBuffer(make([]byte, 0, 256))
	lite.WriteByte(ent.Level.CapitalString()[0])
	if !ent.Time.IsZero() {
		t := ent.Time.AppendFormat(lite.AvailableBuffer(), liteTimeEncoderLayout)
		lite.Write(t)
	}
	lite.WriteString(c.cfg.ConsoleSeparator)
	lite.WriteString(ent.LoggerName)
	if ent.Caller.Defined {
		if ent.LoggerName != "" {
			lite.WriteByte('(')
		}
		lite.WriteString(ent.Caller.TrimmedPath())
		if ent.LoggerName != "" {
			lite.WriteByte(')')
		}
	}
	ent.LoggerName = lite.String()

	return c.Encoder.EncodeEntry(ent, fields)
}

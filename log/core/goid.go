package core

import (
	"github.com/petermattis/goid"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const goidField = "goid"

func NewAddGoidCore(c zapcore.Core) zapcore.Core {
	return &goidCore{c}
}

type goidCore struct {
	zapcore.Core
}

func (c *goidCore) With(fields []zapcore.Field) zapcore.Core {
	return &goidCore{
		c.Core.With(fields),
	}
}

func (c *goidCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	hasGoidField := false
	for _, field := range fields {
		if field.Key == goidField {
			hasGoidField = true
			break
		}
	}
	if !hasGoidField {
		fields = append(fields, zap.Int64(goidField, goid.Get()))
	}
	return c.Core.Write(ent, fields)
}

func (c *goidCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

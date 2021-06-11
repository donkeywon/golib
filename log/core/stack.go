package core

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/bufferpool"

	"go.uber.org/zap/zapcore"
)

func NewStackExtractCore(c zapcore.Core) zapcore.Core {
	return &errStackExtractCore{c}
}

type errStackExtractCore struct {
	zapcore.Core
}

func (c *errStackExtractCore) With(fields []zapcore.Field) zapcore.Core {
	return &errStackExtractCore{
		c.Core.With(fields),
	}
}

func (c *errStackExtractCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	if !hasErr(fields) {
		return c.Core.Write(ent, fields)
	}
	buf := bufferpool.GetBuffer()
	defer buf.Free()
	fields = extractFieldsStacksToBuff(buf, fields)

	if ent.Stack == "" {
		ent.Stack = "error: " + buf.String()
	} else {
		ent.Stack = ent.Stack + "\nerror: " + buf.String()
	}
	return c.Core.Write(ent, fields)
}

func (c *errStackExtractCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func hasErr(fields []zapcore.Field) bool {
	for _, field := range fields {
		if field.Type == zapcore.ErrorType {
			return true
		}
	}
	return false
}

func extractFieldsStacksToBuff(buf *bufferpool.Buffer, fields []zapcore.Field) []zapcore.Field {
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if field.Type != zapcore.ErrorType {
			continue
		}
		fields = append(fields[:i], fields[i+1:]...)

		errs.ErrToStack(field.Interface.(error), buf, 0)

		// only support one error
		break
	}
	return fields
}

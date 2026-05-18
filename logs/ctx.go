package logs

import (
	"context"
	"fmt"
	"log/slog"
)

type ctxKeyLogger struct{}

// CtxWith stores a *slog.Logger into context.
func CtxWith(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger{}, l)
}

// FromCtx retrieves a *slog.Logger from context. Panics if not found.
func FromCtx(ctx context.Context) *slog.Logger {
	v := ctx.Value(ctxKeyLogger{})
	if v == nil {
		panic("no logger from ctx")
	}
	l, ok := v.(*slog.Logger)
	if !ok {
		panic(fmt.Sprintf("unknown logger type from ctx: %T", v))
	}
	return l
}

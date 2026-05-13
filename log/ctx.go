package log

import (
	"context"
	"fmt"
	"reflect"
)

type ctxKeyLogger struct{}

func CtxWith(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger{}, l)
}

func FromCtx(ctx context.Context) Logger {
	a := ctx.Value(ctxKeyLogger{})
	if a == nil {
		panic("no logger from ctx")
	}
	l, ok := a.(Logger)
	if !ok {
		panic(fmt.Sprintf("unknown logger from ctx: %+v", reflect.TypeOf(l)))
	}
	return l
}

package logs

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCtxWith(t *testing.T) {
	t.Run("stores logger in context", func(t *testing.T) {
		ctx := context.Background()
		l := slog.New(slog.DiscardHandler)
		ctxWithLogger := CtxWith(ctx, l)

		// context.Background does not have the logger
		v := ctx.Value(ctxKeyLogger{})
		assert.Nil(t, v)

		// context with logger has it
		v = ctxWithLogger.Value(ctxKeyLogger{})
		require.NotNil(t, v)
		assert.Same(t, l, v)
	})

	t.Run("parent ctx inherits logger from child", func(t *testing.T) {
		ctx := context.Background()
		l := slog.New(slog.DiscardHandler)
		child := CtxWith(ctx, l)

		// Only child has it, parent does not
		v := ctx.Value(ctxKeyLogger{})
		assert.Nil(t, v)

		v = child.Value(ctxKeyLogger{})
		assert.Same(t, l, v)
	})

	t.Run("nil logger is stored successfully", func(t *testing.T) {
		ctx := context.Background()
		ctxWithLogger := CtxWith(ctx, nil)
		v := ctxWithLogger.Value(ctxKeyLogger{})
		// value exists but is nil
		assert.Nil(t, v)
	})
}

func TestFromCtx(t *testing.T) {
	t.Run("retrieves previously stored logger", func(t *testing.T) {
		l := slog.New(slog.DiscardHandler)
		ctx := CtxWith(context.Background(), l)
		got := FromCtx(ctx)
		assert.Same(t, l, got)
	})

	t.Run("panics when no logger in context", func(t *testing.T) {
		ctx := context.Background()
		assert.Panics(t, func() {
			FromCtx(ctx)
		})
	})

	t.Run("panics when wrong type stored under key", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ctxKeyLogger{}, "not a logger")
		assert.Panics(t, func() {
			FromCtx(ctx)
		})
	})
}

func TestCtxRoundTrip(t *testing.T) {
	t.Run("round trip with multiple loggers", func(t *testing.T) {
		l1 := slog.New(slog.DiscardHandler)
		l2 := slog.New(slog.DiscardHandler)

		ctx := context.Background()
		ctx = CtxWith(ctx, l1)
		assert.Same(t, l1, FromCtx(ctx))

		ctx = CtxWith(ctx, l2)
		assert.Same(t, l2, FromCtx(ctx))
	})

	t.Run("FromCtx with multiple keys in context", func(t *testing.T) {
		type otherKey struct{}
		l := slog.New(slog.DiscardHandler)
		ctx := context.Background()
		ctx = context.WithValue(ctx, otherKey{}, "unrelated")
		ctx = CtxWith(ctx, l)
		got := FromCtx(ctx)
		assert.Same(t, l, got)
	})
}

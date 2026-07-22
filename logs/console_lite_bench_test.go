package logs

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

// --- Benchmarks ---

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func makeAttrs(n int) []slog.Attr {
	attrs := make([]slog.Attr, 0, n*3)
	for i := range n {
		attrs = append(attrs,
			slog.String("task_id", "single-log-backup-task-mars-1-mysql-bin.024230.1784796401"),
			slog.String("task_type", "SINGLE_LOG_BACKUP"),
			slog.Int("seq", i),
		)
	}
	return attrs
}

func benchmarkHandler(b *testing.B, h slog.Handler, attrs []slog.Attr, msg string) {
	b.Helper()
	ctx := context.Background()
	var r slog.Record
	for b.Loop() {
		r = slog.NewRecord(r.Time, slog.LevelInfo, msg, 0)
		r.AddAttrs(attrs...)
		h.Handle(ctx, r)
	}
}

func BenchmarkHandler_0attrs(b *testing.B) {
	h := NewConsoleLiteHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelInfo})
	benchmarkHandler(b, h, nil, "init done")
}

func BenchmarkHandler_5attrs(b *testing.B) {
	h := NewConsoleLiteHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelInfo})
	benchmarkHandler(b, h, makeAttrs(5), "init done")
}

func BenchmarkHandler_15attrs(b *testing.B) {
	h := NewConsoleLiteHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelInfo})
	benchmarkHandler(b, h, makeAttrs(15), "init done")
}

func BenchmarkHandler_WithAttrs(b *testing.B) {
	h := NewConsoleLiteHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelInfo})
	h = h.WithAttrs(makeAttrs(3)).(*ConsoleLiteHandler)
	benchmarkHandler(b, h, makeAttrs(5), "init done")
}

// --- vs standard JSON handler ---

func BenchmarkStdJSON_5attrs(b *testing.B) {
	h := slog.NewJSONHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelInfo})
	benchmarkHandler(b, h, makeAttrs(5), "init done")
}

func BenchmarkStdJSON_15attrs(b *testing.B) {
	h := slog.NewJSONHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelInfo})
	benchmarkHandler(b, h, makeAttrs(15), "init done")
}

// --- vs standard Text handler ---

func BenchmarkStdText_5attrs(b *testing.B) {
	h := slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelInfo})
	benchmarkHandler(b, h, makeAttrs(5), "init done")
}

func BenchmarkStdText_15attrs(b *testing.B) {
	h := slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelInfo})
	benchmarkHandler(b, h, makeAttrs(15), "init done")
}

// --- Correctness test ---

func TestHandlerOutput(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleLiteHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(h)
	logger.Info("init done", "task_id", "test-123", "gid", 93)

	output := buf.String()
	// Verify format: IMMDD HH:MM:SS.mmmmmm\tinit done\t{"source":"...",...}
	if !bytes.Contains([]byte(output), []byte("\tinit done\t")) {
		t.Errorf("expected tab-separated format, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte(`"goid":`)) {
		t.Errorf("expected goid attr, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte(`"source":`)) {
		t.Errorf("expected source attr, got: %s", output)
	}
}

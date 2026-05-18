package logging

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"
)

type testLogger struct {
	name string
}

func (w testLogger) Write(p []byte) (n int, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "[%s] %s", w.name, string(p))
	return len(p), nil
}

// NewTestLogger creates an slog.Logger that writes to the test log.
func NewTestLogger(t testing.TB) *slog.Logger {
	t.Helper()
	handler := slog.NewTextHandler(&testLogger{name: t.Name()}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	// Keep standard logger aligned with slog output during tests.
	log.SetOutput(os.Stderr)
	return slog.New(handler)
}

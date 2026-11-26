package storage

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWarnGroupCommitDialect(t *testing.T) {
	tests := map[string]struct {
		dialect string
		warns   bool
	}{
		"postgres": {dialect: "postgres", warns: true},
		"sqlite":   {dialect: "sqlite", warns: false},
		"mysql":    {dialect: "mysql", warns: false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

			warnGroupCommitDialect(logger, test.dialect)

			if test.warns {
				assert.Contains(t, buf.String(), "level=WARN")
				assert.Contains(t, buf.String(), "intended for SQLite")
				return
			}
			assert.Empty(t, buf.String())
		})
	}
}

package database_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
)

func TestSlogGormLogger_Trace(t *testing.T) {
	tests := map[string]struct {
		level      slog.Level
		err        error
		wantRender bool
		wantInLog  []string
	}{
		"debug enabled renders and logs the statement": {
			level:      slog.LevelDebug,
			wantRender: true,
			wantInLog:  []string{`msg="SQL Query"`, `sql="SELECT 1"`, "rows=7"},
		},
		"debug enabled logs the statement error": {
			level:      slog.LevelDebug,
			err:        errors.New("boom"),
			wantRender: true,
			wantInLog:  []string{`sql="SELECT 1"`, "error=boom"},
		},
		"debug disabled does not render the statement": {
			level: slog.LevelInfo,
		},
		"debug disabled does not render a failed statement either": {
			level: slog.LevelWarn,
			err:   errors.New("boom"),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			var out bytes.Buffer
			logger := database.NewSlogGormLogger(slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: test.level})))

			rendered := false
			fc := func() (string, int64) {
				rendered = true
				return "SELECT 1", 7
			}

			// when:
			logger.Trace(context.Background(), time.Now(), fc, test.err)

			// then:
			require.Equal(t, test.wantRender, rendered)
			if !test.wantRender {
				require.Empty(t, out.String())
				return
			}
			for _, want := range test.wantInLog {
				require.Contains(t, out.String(), want)
			}
		})
	}
}

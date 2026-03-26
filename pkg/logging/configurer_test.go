package logging_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

func TestTextLogger(t *testing.T) {
	// given:
	stringWriter := &logging.TestWriter{}
	logger := logging.New().
		WithLevel(defs.LogLevelDebug).
		WithHandler(defs.TextHandler, stringWriter).
		Logger()

	// when:
	logger.Debug("debug message")

	// then:
	msg := stringWriter.String()
	require.Contains(t, msg, "time=")
	require.Contains(t, msg, "level=DEBUG")
	require.Contains(t, msg, `msg="debug message"`)
}

func TestJSONLogger(t *testing.T) {
	// given:
	stringWriter := &logging.TestWriter{}
	logger := logging.New().
		WithLevel(defs.LogLevelDebug).
		WithHandler(defs.JSONHandler, stringWriter).
		Logger()

	// when:
	logger.Debug("debug message")

	// then:
	msg := stringWriter.String()
	require.Contains(t, msg, `"time"`)
	require.Contains(t, msg, `"level":"DEBUG"`)
	require.Contains(t, msg, `"msg":"debug message"`)
}

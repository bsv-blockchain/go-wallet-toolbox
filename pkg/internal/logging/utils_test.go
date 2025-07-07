package logging_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/stretchr/testify/require"
)

func TestChildLogger(t *testing.T) {
	// given:
	stringWriter := &logging.TestWriter{}
	logger := logging.New().
		WithLevel(defs.LogLevelDebug).
		WithHandler(defs.TextHandler, stringWriter).
		Logger()

	// when:
	childLogger := logging.Child(logger, "child")

	// and:
	childLogger.Debug("debug message")

	// then:
	msg := stringWriter.String()
	require.Contains(t, msg, "service=child")
	require.Contains(t, msg, `msg="debug message"`)
}

func TestNopIfNil(t *testing.T) {
	// when:
	logger := logging.DefaultIfNil(nil)

	// then:
	require.NotNil(t, logger)
}

func TestIsDebug(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		// given:
		stringWriter := &logging.TestWriter{}
		logger := logging.New().
			WithLevel(defs.LogLevelDebug).
			WithHandler(defs.TextHandler, stringWriter).
			Logger()

		// when:
		isDebug := logging.IsDebug(logger)

		// then:
		require.True(t, isDebug)
	})

	t.Run("false", func(t *testing.T) {
		// given:
		stringWriter := &logging.TestWriter{}
		logger := logging.New().
			WithLevel(defs.LogLevelInfo).
			WithHandler(defs.TextHandler, stringWriter).
			Logger()

		// when:
		isDebug := logging.IsDebug(logger)

		// then:
		require.False(t, isDebug)
	})
}

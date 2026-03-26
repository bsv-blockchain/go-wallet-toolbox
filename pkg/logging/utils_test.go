package logging_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
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

func TestNumber(t *testing.T) {
	const expectedAttributeForIntegers = "key=42"
	const expectedAttributeForFloat = "key=42.5"
	t.Run("convert uint to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", uint(42))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("convert uint8 to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", uint8(42))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("convert uint16 to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", uint16(42))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("convert uint32 to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", uint32(42))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("convert uint64 to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", uint64(42))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("convert int to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", 42)

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("convert int8 to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", int8(42))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("convert int16 to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", int16(42))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("convert int32 to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", int32(42))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("convert int64 to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", int64(42))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("convert float32 to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", float32(42.5))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForFloat)
	})

	t.Run("convert float64 to slog.Attr", func(t *testing.T) {
		// when:
		attr := logging.Number("key", float64(42.5))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForFloat)
	})

	t.Run("convert custom type over int to slog.Attr", func(t *testing.T) {
		type MyInt int

		// when:
		attr := logging.Number("key", MyInt(42))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("convert custom type over float to slog.Attr", func(t *testing.T) {
		type MyFloat float64

		// when:
		attr := logging.Number("key", MyFloat(42.5))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForFloat)
	})

	t.Run("convert custom type over uint to slog.Attr", func(t *testing.T) {
		type MyUint uint

		// when:
		attr := logging.Number("key", MyUint(42))

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})

	t.Run("satoshis.Value", func(t *testing.T) {
		// given:
		value, err := satoshi.From(42)
		require.NoError(t, err)

		// when:
		attr := logging.Number("key", value)

		// then:
		require.Contains(t, attr.String(), expectedAttributeForIntegers)
	})
}

package primitives_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestBooleanDefaultFalse(t *testing.T) {
	t.Run("nil value returns false", func(t *testing.T) {
		// given:
		var b *primitives.BooleanDefaultFalse

		// when:
		result := b.Value()

		// then:
		require.False(t, result)
	})

	t.Run("false value returns false", func(t *testing.T) {
		// given:
		value := primitives.BooleanDefaultFalse(false)
		b := &value

		// when:
		result := b.Value()

		// then:
		require.False(t, result)
	})

	t.Run("true value returns true", func(t *testing.T) {
		// given:
		value := primitives.BooleanDefaultFalse(true)
		b := &value

		// when:
		result := b.Value()

		// then:
		require.True(t, result)
	})
}

func TestBooleanDefaultTrue(t *testing.T) {
	t.Run("nil value returns true", func(t *testing.T) {
		// given:
		var b *primitives.BooleanDefaultTrue

		// when:
		result := b.Value()

		// then:
		require.True(t, result)
	})

	t.Run("false value returns false", func(t *testing.T) {
		// given:
		value := primitives.BooleanDefaultTrue(false)
		b := &value

		// when:
		result := b.Value()

		// then:
		require.False(t, result)
	})

	t.Run("true value returns true", func(t *testing.T) {
		// given:
		value := primitives.BooleanDefaultTrue(true)
		b := &value

		// when:
		result := b.Value()

		// then:
		require.True(t, result)
	})
}

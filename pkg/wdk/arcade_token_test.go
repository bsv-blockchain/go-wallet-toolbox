package wdk_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestDeriveArcadeCallbackToken(t *testing.T) {
	t.Run("token is 64 hex chars", func(t *testing.T) {
		token := wdk.DeriveArcadeCallbackToken("02abc123")
		require.Len(t, token, 64)
	})

	t.Run("deterministic for same key", func(t *testing.T) {
		key := "02deadbeef"
		token1 := wdk.DeriveArcadeCallbackToken(key)
		token2 := wdk.DeriveArcadeCallbackToken(key)
		require.Equal(t, token1, token2)
	})

	t.Run("different for different keys", func(t *testing.T) {
		token1 := wdk.DeriveArcadeCallbackToken("02aaaa")
		token2 := wdk.DeriveArcadeCallbackToken("02bbbb")
		require.NotEqual(t, token1, token2)
	})

	t.Run("known value", func(t *testing.T) {
		// Pins the HMAC key/message orientation and domain string — change here = breaking change.
		require.Equal(t, "4dfb4c073ef83056a7301f09f42b86ceb39074b0011cff5ba11670d5888836db", wdk.DeriveArcadeCallbackToken("02abc123"))
	})
}

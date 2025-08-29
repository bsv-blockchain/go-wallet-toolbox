package testabilities

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func RandomTxID(t testing.TB) string {
	t.Helper()
	return "txid_" + RandHex(t, 32)
}

func RandHex(t testing.TB, bytes int) string {
	t.Helper()
	b := make([]byte, bytes)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return hex.EncodeToString(b)
}

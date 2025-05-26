package testutils

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"
	"testing"
)

func BEEFFromHex(t testing.TB, beefBytes []byte) *transaction.Beef {
	t.Helper()

	beef, err := transaction.NewBeefFromBytes(beefBytes)
	require.NoError(t, err)

	return beef
}

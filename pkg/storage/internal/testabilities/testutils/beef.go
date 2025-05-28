package testutils

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"
)

func BEEFFromHex(t testing.TB, beefBytes []byte) *transaction.Beef {
	t.Helper()

	beef, err := transaction.NewBeefFromBytes(beefBytes)
	require.NoError(t, err)

	return beef
}

package testutils

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"
)

func BEEFFromHex(t testing.TB, beefInts []int) *transaction.Beef {
	t.Helper()

	beefBytes := make([]byte, len(beefInts))
	for i, b := range beefInts {
		beefBytes[i] = byte(b)
	}

	beef, err := transaction.NewBeefFromBytes(beefBytes)
	require.NoError(t, err)

	return beef
}

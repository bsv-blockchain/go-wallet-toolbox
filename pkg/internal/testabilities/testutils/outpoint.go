package testutils

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"
)

func SdkOutpoint(t testing.TB, strOutpoint string) *transaction.Outpoint {
	t.Helper()
	outpoint, err := transaction.OutpointFromString(strOutpoint)
	require.NoError(t, err)
	return outpoint
}

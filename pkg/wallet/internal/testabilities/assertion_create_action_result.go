package testabilities

import (
	stdslices "slices"
	"testing"

	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
)

type SendWithResultsAsserter []wallet.SendWithResult

func (r SendWithResultsAsserter) ContainsTxsWithStatus(t *testing.T, status wallet.ActionResultStatus, txIDs ...string) {
	for _, txID := range txIDs {
		idx := stdslices.IndexFunc(r, func(i wallet.SendWithResult) bool {
			return i.Txid.String() == txID
		})

		assert.GreaterOrEqual(t, idx, 0, "Expected to find txID %s in send with results", txID)
		assert.Equal(t, status, r[idx].Status, "Expected status for txID %s to be %s, got %s", txID, status, r[idx].Status)
	}
}

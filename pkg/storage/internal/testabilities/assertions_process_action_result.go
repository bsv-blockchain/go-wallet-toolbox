package testabilities

import (
	stdslices "slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

type NotDelayedResultsAsserter []wdk.ReviewActionResult

func (n NotDelayedResultsAsserter) ContainsTxsWithStatus(t testing.TB, status wdk.ReviewActionResultStatus, txIDs ...string) {
	for _, txID := range txIDs {
		idx := stdslices.IndexFunc(n, func(i wdk.ReviewActionResult) bool {
			return i.TxID == primitives.TXIDHexString(txID)
		})

		assert.GreaterOrEqual(t, idx, 0, "Expected to find txID %s in not delayed results", txID)
		assert.Equal(t, status, n[idx].Status, "Expected status for txID %s to be %s, got %s", txID, status, n[idx].Status)
	}
}

type SendWithResultsAsserter []wdk.SendWithResult

func (r SendWithResultsAsserter) ContainsTxsWithStatus(t testing.TB, status wdk.SendWithResultStatus, txIDs ...string) {
	for _, txID := range txIDs {
		idx := stdslices.IndexFunc(r, func(i wdk.SendWithResult) bool {
			return i.TxID == primitives.TXIDHexString(txID)
		})

		assert.GreaterOrEqual(t, idx, 0, "Expected to find txID %s in send with results", txID)
		assert.Equal(t, status, r[idx].Status, "Expected status for txID %s to be %s, got %s", txID, status, r[idx].Status)
	}
}

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// A pre-broadcast abort must only touch rows the caller owns, and must back off entirely
// when any other row keeps the shared transaction alive.
func TestSplitTxsForPreBroadcastAbort(t *testing.T) {
	const (
		alice = 1
		bob   = 2
	)

	tests := map[string]struct {
		transactions      []*pkgentity.Transaction
		userScope         *int
		expectedToAbort   []uint
		expectOthersAlive bool
	}{
		"single owner in an abortable status": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusUnprocessed},
			},
			userScope:       ptr(alice),
			expectedToAbort: []uint{10},
		},
		"row of another user is neither aborted nor ignored": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusUnprocessed},
				{ID: 11, UserID: bob, Status: wdk.TxStatusUnprocessed},
			},
			userScope:         ptr(alice),
			expectedToAbort:   []uint{10},
			expectOthersAlive: true,
		},
		"already aborted rows of other users do not keep the tx alive": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusUnprocessed},
				{ID: 11, UserID: bob, Status: wdk.TxStatusAborted},
				{ID: 12, UserID: bob, Status: wdk.TxStatusFailed},
			},
			userScope:       ptr(alice),
			expectedToAbort: []uint{10},
		},
		"sending is not abortable and keeps the tx alive": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusSending},
			},
			userScope:         ptr(alice),
			expectedToAbort:   nil,
			expectOthersAlive: true,
		},
		"broadcast rows are never abortable": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusUnproven},
			},
			userScope:         ptr(alice),
			expectedToAbort:   nil,
			expectOthersAlive: true,
		},
		"storage-wide sweep covers every owner": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusUnprocessed},
				{ID: 11, UserID: bob, Status: wdk.TxStatusUnprocessed},
			},
			userScope:       nil,
			expectedToAbort: []uint{10, 11},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			p := &process{}

			// when:
			toAbort, othersStillActive := p.splitTxsForPreBroadcastAbort(test.transactions, test.userScope)

			// then:
			assert.Equal(t, test.expectedToAbort, toAbort)
			assert.Equal(t, test.expectOthersAlive, othersStillActive)
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}

package txutils_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
)

// beefWith merges the given transactions into one BEEF, in the order they are handed over.
func beefWith(t *testing.T, txs ...*transaction.Transaction) *transaction.Beef {
	t.Helper()
	beef := transaction.NewBeefV2()
	for _, tx := range txs {
		_, err := beef.MergeRawTx(tx.Bytes(), nil)
		require.NoError(t, err)
	}
	return beef
}

// TestParentsFirst covers the contract both broadcast paths depend on: PostFromBEEF posts a
// slice in order and Arcade forwards upstream in receive order, so a child spending another
// subject of the same post has to come after it - while everything else stays where it was,
// because that input order carries the transactions' creation order.
func TestParentsFirst(t *testing.T) {
	parentSpec := txtestabilities.GivenTX().WithInput(200_000).WithP2PKHOutput(150_000)
	childSpec := txtestabilities.GivenTX().
		WithSender(txtestabilities.Bob).
		WithInputFromUTXO(parentSpec.TX(), 0).
		WithP2PKHOutput(100_000)
	grandchildSpec := txtestabilities.GivenTX().
		WithSender(txtestabilities.Bob).
		WithInputFromUTXO(childSpec.TX(), 0).
		WithP2PKHOutput(50_000)

	parent, child, grandchild := parentSpec.ID().String(), childSpec.ID().String(), grandchildSpec.ID().String()
	beef := beefWith(t, parentSpec.TX(), childSpec.TX(), grandchildSpec.TX())

	t.Run("moves a child behind the parent it spends", func(t *testing.T) {
		assert.Equal(t, []string{parent, child}, txutils.ParentsFirst(beef, []string{child, parent}))
	})

	t.Run("resolves a chain, not just a pair", func(t *testing.T) {
		assert.Equal(t, []string{parent, child, grandchild}, txutils.ParentsFirst(beef, []string{grandchild, child, parent}))
	})

	t.Run("leaves an order that already respects the dependencies alone", func(t *testing.T) {
		ordered := []string{parent, child, grandchild}
		assert.Equal(t, ordered, txutils.ParentsFirst(beef, ordered))
	})

	t.Run("does not modify the input slice", func(t *testing.T) {
		input := []string{child, parent}
		txutils.ParentsFirst(beef, input)
		assert.Equal(t, []string{child, parent}, input)
	})

	t.Run("keeps unrelated transactions in the order they were given", func(t *testing.T) {
		// Only the child has to move; the parent's position relative to an unrelated
		// transaction must survive, otherwise the creation order would not.
		unrelatedSpec := txtestabilities.GivenTX().WithInput(90_000).WithP2PKHOutput(80_000)
		unrelated := unrelatedSpec.ID().String()
		withUnrelated := beefWith(t, parentSpec.TX(), childSpec.TX(), unrelatedSpec.TX())

		assert.Equal(t,
			[]string{parent, child, unrelated},
			txutils.ParentsFirst(withUnrelated, []string{parent, child, unrelated}),
		)
		assert.Equal(t,
			[]string{parent, unrelated, child},
			txutils.ParentsFirst(withUnrelated, []string{parent, unrelated, child}),
		)
	})

	t.Run("passes through what it cannot reorder", func(t *testing.T) {
		assert.Equal(t, []string{child}, txutils.ParentsFirst(beef, []string{child}))
		assert.Equal(t, []string{child, parent}, txutils.ParentsFirst(nil, []string{child, parent}))
		assert.Empty(t, txutils.ParentsFirst(beef, nil))
	})
}

package wallet_test

import (
	"context"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestListOutputsOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(
		t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.ListOutputsResult, error) {
			args := fixtures.DefaultWalletListOutputsArgs()
			return w.ListOutputs(ctx, args, originator)
		},
	)
}

func TestWalletListOutputsArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.ListOutputsArgs
	}{
		"invalid limit (too high)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.ListOutputsArgs {
				args := fixtures.DefaultWalletListOutputsArgs()
				args.Limit = to.Ptr[uint32](10001)
				return args
			},
		},
		"invalid limit (zero)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.ListOutputsArgs {
				args := fixtures.DefaultWalletListOutputsArgs()
				args.Limit = to.Ptr[uint32](0)
				return args
			},
		},
	}

	for name, test := range errorTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			// and:
			aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

			// when:
			result, err := aliceWallet.ListOutputs(t.Context(), test.args(), test.originator)

			// then:
			then.Result(result).HasError(err)

			then.Storage().HadNoInteraction()
		})
	}
}

const shouldHaveAtLeastOneOutputMsg = "Should have at least one output after internalize"

func (s *WalletTestSuite) TestWalletListOutputs() {
	s.Run("list outputs with empty result when no outputs exist", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.Empty(t, result.Outputs, "Should have no outputs when none exist")
		assert.Equal(t, uint32(0), result.TotalOutputs, "Total outputs should be zero")
	})

	s.Run("basic list outputs after internalize action", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgsMatchingBRC29(t, sdk.InternalizeProtocolWalletPayment, testusers.Alice.KeyDeriver(t))
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.NotEmpty(t, result.Outputs, shouldHaveAtLeastOneOutputMsg)
		assert.Equal(t, uint64(fixtures.ExpectedValueToInternalize), result.Outputs[0].Satoshis, "Output value should match internalized amount")
	})

	s.Run("list outputs with custom limit after internalize action", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgsMatchingBRC29(t, sdk.InternalizeProtocolWalletPayment, testusers.Alice.KeyDeriver(t))
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Limit = to.Ptr[uint32](50)

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.NotEmpty(t, result.Outputs, shouldHaveAtLeastOneOutputMsg)
	})

	s.Run("list outputs with include entire transactions after internalize action", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgsMatchingBRC29(t, sdk.InternalizeProtocolWalletPayment, testusers.Alice.KeyDeriver(t))
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Include = sdk.OutputIncludeEntireTransactions

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.NotEmpty(t, result.Outputs, shouldHaveAtLeastOneOutputMsg)
		assert.NotNil(t, result.BEEF, "BEEF should be included when requesting entire transactions")
	})

	s.Run("repeated list outputs with entire transactions returns resolvable BEEF", func() {
		t := s.T()

		// The first call teaches the wallet's beef party about these
		// transactions, so the second advertises them as known and storage
		// answers with txid-only stubs. The wallet has to resolve those back
		// into full transactions before returning them to the caller.
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgsMatchingBRC29(t, sdk.InternalizeProtocolWalletPayment, testusers.Alice.KeyDeriver(t))
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Include = sdk.OutputIncludeEntireTransactions

		// when:
		first, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, first.BEEF, "BEEF should be included when requesting entire transactions")

		// when:
		second, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err, "a second call must not fail resolving known transactions")
		require.NotEmpty(t, second.BEEF, "BEEF should still be returned once transactions are known")
		assert.Len(t, second.Outputs, len(first.Outputs))

		// and: every returned outpoint is backed by a full transaction, not a stub
		beef, err := transaction.NewBeefFromBytes(second.BEEF)
		require.NoError(t, err)
		for _, output := range second.Outputs {
			txID := output.Outpoint.Txid

			btx, ok := beef.Transactions[txID]
			require.Truef(t, ok, "returned BEEF is missing tx %s", txID)
			assert.NotEqualf(t, transaction.TxIDOnly, btx.DataFormat,
				"tx %s came back as a txid-only stub the caller cannot use", txID)
		}
	})

	s.Run("list outputs with include locking scripts after internalize action", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgsMatchingBRC29(t, sdk.InternalizeProtocolWalletPayment, testusers.Alice.KeyDeriver(t))
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Include = sdk.OutputIncludeLockingScripts

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.NotEmpty(t, result.Outputs, shouldHaveAtLeastOneOutputMsg)
		assert.NotNil(t, result.Outputs[0].LockingScript, "Locking script should be included")
		assert.NotEmpty(t, result.Outputs[0].LockingScript, "Locking script should not be empty")
	})

	s.Run("list outputs with basket insertion protocol", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and: first internalize an action using basket insertion protocol
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolBasketInsertion)
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and: list outputs from the custom basket
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Basket = fixtures.CustomBasket
		trueValue := true
		args.IncludeTags = &trueValue
		args.IncludeLabels = &trueValue
		args.IncludeCustomInstructions = &trueValue

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.NotEmpty(t, result.Outputs, "Should have at least one output in custom basket")
		assert.Equal(t, uint64(fixtures.ExpectedValueToInternalize), result.Outputs[0].Satoshis, "Output value should match internalized amount")

		// and:
		assert.NotNil(t, result.Outputs[0].Tags, "Tags should be included")
		assert.NotEmpty(t, result.Outputs[0].Tags, "Should have tags")
		assert.Contains(t, result.Outputs[0].Tags, "tag1", "Should contain expected tag")
		assert.Contains(t, result.Outputs[0].Tags, "tag2", "Should contain expected tag")
		assert.NotEmpty(t, result.Outputs[0].CustomInstructions, "Custom instructions should be included")

		// and: includeLabels=true should populate transaction labels on each output
		assert.NotEmpty(t, result.Outputs[0].Labels, "Labels should be included when IncludeLabels=true")
		assert.Contains(t, result.Outputs[0].Labels, "label1", "Should contain expected label")
		assert.Contains(t, result.Outputs[0].Labels, "label2", "Should contain expected label")

		// when: includeLabels=false
		falseValue := false
		args.IncludeLabels = &falseValue
		resultWithoutLabels, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.NotEmpty(t, resultWithoutLabels.Outputs)
		assert.Empty(t, resultWithoutLabels.Outputs[0].Labels, "Labels should be empty when IncludeLabels=false")
	})
}

// Resolving the txid-only stubs storage sends back is an optimisation, not a
// correctness requirement: the BEEF storage sent is a valid reply on its own.
// So a stub the wallet cannot resolve must degrade to that reply, the way
// CreateAction already does, instead of failing a read the caller would only
// retry.
//
// This is the shape the wallet ends up in when the shared graph is dropped out
// from under an in-flight call - the bound has a hard ceiling that fires even
// while leases are open (wdk.EmergencyResetFactor), which under sustained
// concurrency leaves a reply asking for transactions the graph no longer has.
func (s *WalletTestSuite) TestListOutputsFallsBackWhenAStubCannotBeResolved() {
	t := s.T()

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	aliceWallet := given.AliceWalletWithStorage(s.StorageType)

	// and:
	internalizeArgs := fixtures.DefaultWalletInternalizeActionArgsMatchingBRC29(t, sdk.InternalizeProtocolWalletPayment, testusers.Alice.KeyDeriver(t))
	_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
	require.NoError(t, err, "Failed to internalize action for test setup")

	// and:
	_, subject, err := transaction.NewBeefFromAtomicBytes(internalizeArgs.Tx)
	require.NoError(t, err)

	// and: the party holds that transaction as a bare txid under a proof, which
	// is what the known-txid list is built from but not what resolving a reply
	// accepts - so the wallet advertises a transaction it cannot produce.
	stranded := transaction.NewBeef()
	stranded.MergeTxidOnly(subject)
	proof := testutils.MockValidMerklePath(t, subject.String(), 800_000)
	stranded.MergeBump(&proof)
	strandedBytes, err := stranded.Bytes()
	require.NoError(t, err)
	require.NoError(t, aliceWallet.GetBeefParty().MergeBeefFromParty(t.Context(), storagePartyID, strandedBytes))

	// and:
	args := fixtures.DefaultWalletListOutputsArgs()
	args.Include = sdk.OutputIncludeEntireTransactions

	// when:
	result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

	// then:
	require.NoError(t, err, "an unresolvable stub must not fail the call")
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Outputs, shouldHaveAtLeastOneOutputMsg)
	require.NotEmpty(t, result.BEEF, "the BEEF storage sent must still be returned")

	// and: what comes back is storage's own reply, stub and all
	beef, err := transaction.NewBeefFromBytes(result.BEEF)
	require.NoError(t, err)
	btx, ok := beef.Transactions[*subject]
	require.Truef(t, ok, "returned BEEF is missing tx %s", subject)
	assert.Equal(t, transaction.TxIDOnly, btx.DataFormat,
		"storage answered with a stub, so that is what the fallback returns")
}

package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

func TestNoSendPlusSendWithScenario(t *testing.T) {
	t.Run("two no-send txs, many initial UTXOs, all noSendChange is used, no newTx when sendWith", func(t *testing.T) {
		// given:
		const inputSatoshis = 99904
		const noSendChainCount = 2

		given, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := given.Provider().GORM()
		givenNoSend := testabilities.GivenNoSend(t, given, activeStorage, testusers.Alice)

		// and:
		givenNoSend.FundWallet(inputSatoshis) // This makes the wallet not-empty with several UTXOs

		// when:
		var noSendChangeOutpoints []wdk.OutPoint
		for i := range noSendChainCount {
			t.Logf("Creating NoSend tx #%d", i+1)
			noSendChangeOutpoints = givenNoSend.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		}

		// and:
		// Call processAction using sendWith and IsNewTx set to false, including the two previous transactions in SendWithSlice.
		sendWithProcessAction := givenNoSend.ProcessAction(wdk.ProcessActionArgs{
			IsNewTx:    false,
			IsNoSend:   false,
			SendWith:   givenNoSend.NoSendTxsHexStrings(),
			IsSendWith: true,
		})

		// then:
		testabilities.NotDelayedResultsAsserter(sendWithProcessAction.NotDelayedResults).
			ContainsTxsWithStatus(t, wdk.ReviewActionResultStatusSuccess, givenNoSend.NoSendTxs()...)

		testabilities.SendWithResultsAsserter(sendWithProcessAction.SendWithResults).
			ContainsTxsWithStatus(t, wdk.SendWithResultStatusUnproven, givenNoSend.NoSendTxs()...)

		testabilities.
			ThenDBState(t, activeStorage).
			HasUserTransactionsByTxIDsWithStatus(testusers.Alice, wdk.TxStatusUnproven, givenNoSend.NoSendTxs()...)

		testabilities.ThenFunds(t, testusers.Alice, activeStorage).
			ShouldBeAbleToReserveSatoshis(
				inputSatoshis +
					-4 + // two no-send txs (2 sats each)
					-7, // fee to create a new transaction with many inputs
			)
	})

	t.Run("three no-send txs, single initial UTXO, all noSendChange is used making chain of txs, no newTx when sendWith", func(t *testing.T) {
		// given:
		const inputSatoshis = 6
		const noSendChainCount = 3

		given, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := given.Provider().GORM()
		givenNoSend := testabilities.GivenNoSend(t, given, activeStorage, testusers.Alice)

		// and:
		given.Faucet(activeStorage, testusers.Alice).TopUp(inputSatoshis) // Intentionally, create only one UTXO

		// when:
		var noSendChangeOutpoints []wdk.OutPoint
		for i := range noSendChainCount {
			t.Logf("Creating NoSend tx #%d", i+1)
			noSendChangeOutpoints = givenNoSend.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		}

		// and:
		// Call processAction using sendWith and IsNewTx set to false, including the two previous transactions in SendWithSlice.
		sendWithProcessAction := givenNoSend.ProcessAction(wdk.ProcessActionArgs{
			IsNewTx:    false,
			IsNoSend:   false,
			SendWith:   givenNoSend.NoSendTxsHexStrings(),
			IsSendWith: true,
		})

		// then:
		testabilities.NotDelayedResultsAsserter(sendWithProcessAction.NotDelayedResults).
			ContainsTxsWithStatus(t, wdk.ReviewActionResultStatusSuccess, givenNoSend.NoSendTxs()...)

		testabilities.SendWithResultsAsserter(sendWithProcessAction.SendWithResults).
			ContainsTxsWithStatus(t, wdk.SendWithResultStatusUnproven, givenNoSend.NoSendTxs()...)

		testabilities.
			ThenDBState(t, activeStorage).
			HasUserTransactionsByTxIDsWithStatus(testusers.Alice, wdk.TxStatusUnproven, givenNoSend.NoSendTxs()...)

		testabilities.ThenFunds(t, testusers.Alice, activeStorage).
			ShouldNotBeAbleToReserveSatoshis(1) // All funds are tied up in the long NoSend chain
	})

	t.Run("two no-send txs, two initial UTXOs (one used for txs chain), not all noSendChange used, no newTx when sendWith", func(t *testing.T) {
		// NOTE: In this case, the balance after all should be made of one initial UTXO + not used noSendChanges + last no-send tx change
		// given:
		const initialUTXOSats = 10000
		const noSendChainCount = 2

		given, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := given.Provider().GORM()
		givenNoSend := testabilities.GivenNoSend(t, given, activeStorage, testusers.Alice)

		// and:
		faucet := given.Faucet(activeStorage, testusers.Alice)
		faucet.TopUp(initialUTXOSats)
		faucet.TopUp(initialUTXOSats)

		// when:
		var noSendChangeOutpoints []wdk.OutPoint
		for i := range noSendChainCount {
			t.Logf("Creating NoSend tx #%d", i+1)
			noSendChangeOutpoints = givenNoSend.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		}

		// and:
		// Call processAction using sendWith and IsNewTx set to false, including the two previous transactions in SendWithSlice.
		sendWithProcessAction := givenNoSend.ProcessAction(wdk.ProcessActionArgs{
			IsNewTx:    false,
			IsNoSend:   false,
			SendWith:   givenNoSend.NoSendTxsHexStrings(),
			IsSendWith: true,
		})

		// then:
		testabilities.NotDelayedResultsAsserter(sendWithProcessAction.NotDelayedResults).
			ContainsTxsWithStatus(t, wdk.ReviewActionResultStatusSuccess, givenNoSend.NoSendTxs()...)

		testabilities.SendWithResultsAsserter(sendWithProcessAction.SendWithResults).
			ContainsTxsWithStatus(t, wdk.SendWithResultStatusUnproven, givenNoSend.NoSendTxs()...)

		testabilities.
			ThenDBState(t, activeStorage).
			HasUserTransactionsByTxIDsWithStatus(testusers.Alice, wdk.TxStatusUnproven, givenNoSend.NoSendTxs()...)

		testabilities.ThenFunds(t, testusers.Alice, activeStorage).
			ShouldBeAbleToReserveSatoshis(initialUTXOSats - noSendChainCount*2)
	})

	t.Run("two no-send txs, many initial UTXOs, all noSendChange used, additional UTXO needed, no newTx when sendWith", func(t *testing.T) {
		// given:
		const inputSatoshis = 99904
		const largerUTXOToSend = 50_000

		given, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := given.Provider().GORM()
		givenNoSend := testabilities.GivenNoSend(t, given, activeStorage, testusers.Alice)

		// and:
		givenNoSend.FundWallet(inputSatoshis) // This makes the wallet not-empty with several UTXOs

		// when:
		// step 1:
		var noSendChangeOutpoints []wdk.OutPoint
		noSendChangeOutpoints = givenNoSend.CreateAndProcessNoSendAction(noSendChangeOutpoints)

		// and:
		// step 2:
		givenNoSend.WillSendSats(largerUTXOToSend)
		_ = givenNoSend.CreateAndProcessNoSendAction(noSendChangeOutpoints)

		// and:
		// Call processAction using sendWith and IsNewTx set to false, including the two previous transactions in SendWithSlice.
		sendWithProcessAction := givenNoSend.ProcessAction(wdk.ProcessActionArgs{
			IsNewTx:    false,
			IsNoSend:   false,
			SendWith:   givenNoSend.NoSendTxsHexStrings(),
			IsSendWith: true,
		})

		// then:
		testabilities.NotDelayedResultsAsserter(sendWithProcessAction.NotDelayedResults).
			ContainsTxsWithStatus(t, wdk.ReviewActionResultStatusSuccess, givenNoSend.NoSendTxs()...)

		testabilities.SendWithResultsAsserter(sendWithProcessAction.SendWithResults).
			ContainsTxsWithStatus(t, wdk.SendWithResultStatusUnproven, givenNoSend.NoSendTxs()...)

		testabilities.
			ThenDBState(t, activeStorage).
			HasUserTransactionsByTxIDsWithStatus(testusers.Alice, wdk.TxStatusUnproven, givenNoSend.NoSendTxs()...)

		testabilities.ThenFunds(t, testusers.Alice, activeStorage).
			ShouldBeAbleToReserveSatoshis(
				inputSatoshis +
					-2 + // first no-send tx (2 sats)
					-(largerUTXOToSend + 1) + // second no-send tx (1 sat for fee)
					-7, // fee to create a new transaction with many inputs
			)
	})

	t.Run("complex case, three no-send txs, many initial UTXOs, all noSendChange used, additional UTXOs needed, no newTx when sendWith", func(t *testing.T) {
		// NOTE: In this case, the balance after all should be made of not used initial UTXOs + last no-send tx change
		// Initially, we have many UTXOs.
		// First no-send tx produces several change outputs.
		// Second no-send tx uses ONE of the previous change outputs and produces one change output.
		// Third no-send tx uses noSendChange outputs from all previous txs, but that is not enough, so it needs to select additional UTXOs from the wallet

		// given:
		const inputSatoshis = 100_000
		const largerUTXOToSend = 50_000

		given, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()
		givenNoSend := testabilities.GivenNoSend(t, given, activeStorage, testusers.Alice)

		// and:
		givenNoSend.FundWallet(inputSatoshis) // This makes the wallet not-empty with several UTXOs

		// NOTE: We need to reconfigure the basket to increase the number of UTXOs in the wallet
		// This way, createAction will produce more-than-one change outputs
		err := activeStorage.ConfigureBasket(t.Context(), testusers.Alice.AuthID(), wdk.BasketConfiguration{
			Name:                    wdk.BasketNameForChange,
			NumberOfDesiredUTXOs:    100,
			MinimumDesiredUTXOValue: 1000,
		})
		require.NoError(t, err)

		// when:
		// step 1:
		noSendChangeOutpoints := givenNoSend.CreateAndProcessNoSendAction(nil)
		require.Greater(t, len(noSendChangeOutpoints), 1, "there should be multiple nosend change outpoints")

		// and:
		// step 2:
		noSendChangeOutpoints = givenNoSend.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		require.Equal(t, 1, len(noSendChangeOutpoints), "only one change output should be produced")
		require.Equal(t, 1, givenNoSend.LastUsedChangeOutputsCounter(), "only one change output should be used")
		allRemainedNoSendChangeBeforeStep3 := len(givenNoSend.AllRemainedNoSendChange())
		require.Equal(t, 3, allRemainedNoSendChangeBeforeStep3, "three no-send change outputs should remain")

		// and:
		// step 3:
		givenNoSend.WillSendSats(largerUTXOToSend)
		noSendChangeOutpoints = givenNoSend.CreateAndProcessNoSendAction(givenNoSend.AllRemainedNoSendChange())
		require.Equal(t, 2, len(noSendChangeOutpoints), "two change outputs should be produced")
		require.Len(t, givenNoSend.AllRemainedNoSendChange(), len(noSendChangeOutpoints), "only one no-send change outputs should be the output of the last tx")
		require.Equal(t, allRemainedNoSendChangeBeforeStep3, givenNoSend.LastUsedChangeOutputsCounter(), "all previous no-send change outputs should be used")
		funderSelectedExtraUtxoOutOfNosendPool := len(givenNoSend.LastCreateActionResult().Inputs) > givenNoSend.LastUsedChangeOutputsCounter()
		require.True(t, funderSelectedExtraUtxoOutOfNosendPool, "funder should select at least one extra UTXO outside of no-send change outputs")

		// and:
		// Call processAction using sendWith and IsNewTx set to false, including the two previous transactions in SendWithSlice.
		sendWithProcessAction := givenNoSend.ProcessAction(wdk.ProcessActionArgs{
			IsNewTx:    false,
			IsNoSend:   false,
			SendWith:   givenNoSend.NoSendTxsHexStrings(),
			IsSendWith: true,
		})

		// then:
		testabilities.NotDelayedResultsAsserter(sendWithProcessAction.NotDelayedResults).
			ContainsTxsWithStatus(t, wdk.ReviewActionResultStatusSuccess, givenNoSend.NoSendTxs()...)

		testabilities.SendWithResultsAsserter(sendWithProcessAction.SendWithResults).
			ContainsTxsWithStatus(t, wdk.SendWithResultStatusUnproven, givenNoSend.NoSendTxs()...)

		testabilities.
			ThenDBState(t, activeStorage).
			HasUserTransactionsByTxIDsWithStatus(testusers.Alice, wdk.TxStatusUnproven, givenNoSend.NoSendTxs()...)

		testabilities.ThenFunds(t, testusers.Alice, activeStorage).
			ShouldBeAbleToReserveSatoshis(
				inputSatoshis +
					-3 + // first no-send tx (2 sats for fee)
					-2 + // second no-send tx (1 sats for fee)
					-(largerUTXOToSend + 1) + // third no-send tx (1 sat for fee)
					-7, // fee to create a new transaction with many inputs
			)
	})
}

func TestNoSendPlusSendWithScenario_SendWithNewTx(t *testing.T) {
	// given:
	const inputSatoshis = 99904

	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	givenNoSend := testabilities.GivenNoSend(t, given, activeStorage, testusers.Alice)

	// and:
	givenNoSend.FundWallet(inputSatoshis) // This makes the wallet not-empty with several UTXOs

	// when:
	// step 1:
	noSendChangeOutpoints := givenNoSend.CreateAndProcessNoSendAction(nil)
	require.NotEmpty(t, noSendChangeOutpoints)

	// and:
	// step 2:
	noSendChangeOutpoints = givenNoSend.CreateAndProcessNoSendAction(noSendChangeOutpoints)
	require.NotEmpty(t, noSendChangeOutpoints)

	// and:
	// Call createAction with NoSendChange AND SendWith
	// NOTE: While it is technically possible on storage level, this is not possible on wallet level,
	// because here we provide NoSend = true for createAction and NoSend = false for processAction
	thirdProcessActionResult, thirdTxID := givenNoSend.CreateAndProcessSendWithAction(
		givenNoSend.NoSendTxsHexStrings(),
		givenNoSend.CreateActionNoSendArgsModifier(noSendChangeOutpoints, true),
		givenNoSend.CreateActionSendWithArgsModifier(givenNoSend.NoSendTxsHexStrings()...),
	)

	// then:
	txIDsThatShouldBeBroadcasted := append(givenNoSend.NoSendTxs(), thirdTxID)
	testabilities.NotDelayedResultsAsserter(thirdProcessActionResult.NotDelayedResults).
		ContainsTxsWithStatus(t, wdk.ReviewActionResultStatusSuccess, txIDsThatShouldBeBroadcasted...)

	testabilities.SendWithResultsAsserter(thirdProcessActionResult.SendWithResults).
		ContainsTxsWithStatus(t, wdk.SendWithResultStatusUnproven, txIDsThatShouldBeBroadcasted...)

	testabilities.
		ThenDBState(t, activeStorage).
		HasUserTransactionsByTxIDsWithStatus(testusers.Alice, wdk.TxStatusUnproven, txIDsThatShouldBeBroadcasted...)
}

func TestNoSendSendWithScenario_SendWithSeparatedNewTx(t *testing.T) {
	const inputSatoshis = 99904

	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	givenNoSend := testabilities.GivenNoSend(t, given, activeStorage, testusers.Alice)

	// and:
	givenNoSend.FundWallet(inputSatoshis) // This makes the wallet not-empty with several UTXOs

	// when:
	// step 1:
	noSendChangeOutpoints := givenNoSend.CreateAndProcessNoSendAction(nil)
	require.NotEmpty(t, noSendChangeOutpoints)

	// and:
	// step 2:
	noSendChangeOutpoints = givenNoSend.CreateAndProcessNoSendAction(noSendChangeOutpoints)
	require.NotEmpty(t, noSendChangeOutpoints)

	// and:
	thirdProcessActionResult, thirdTxID := givenNoSend.CreateAndProcessSendWithAction(
		givenNoSend.NoSendTxsHexStrings(),
		givenNoSend.CreateActionNoSendArgsModifier(nil, false), // NOTE the nil as prevNoSendOutpoints, this makes this new tx out of chain of first-second NoSend txs
		givenNoSend.CreateActionSendWithArgsModifier(givenNoSend.NoSendTxsHexStrings()...),
	)

	// then:
	txIDsThatShouldBeTried := append(givenNoSend.NoSendTxs(), thirdTxID)

	// NOTE: ServiceError is because only working broadcaster for unit tests is ARC which does not support sending BEEFs that have multiple subject transactions
	testabilities.NotDelayedResultsAsserter(thirdProcessActionResult.NotDelayedResults).
		ContainsTxsWithStatus(t, wdk.ReviewActionResultStatusServiceError, txIDsThatShouldBeTried...)

	testabilities.SendWithResultsAsserter(thirdProcessActionResult.SendWithResults).
		ContainsTxsWithStatus(t, wdk.SendWithResultStatusSending, txIDsThatShouldBeTried...)

	testabilities.
		ThenDBState(t, activeStorage).
		HasUserTransactionsByTxIDsWithStatus(testusers.Alice, wdk.TxStatusSending, txIDsThatShouldBeTried...)
}

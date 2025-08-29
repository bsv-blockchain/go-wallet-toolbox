package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities/nosendtest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

func TestNoSendPlusSendWithScenario(t *testing.T) {
	t.Run("two no-send txs, many initial UTXOs, all noSendChange is used, no newTx when sendWith", func(t *testing.T) {
		// given:
		const inputSatoshis = 99904
		const noSendChainCount = 2

		givenStorage, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := givenStorage.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()
		given, when, then := nosendtest.New(t, testusers.Alice, givenStorage, activeStorage)

		// and:
		given.UserOwnsMultipleUTXOsToSpend(inputSatoshis)

		// when:
		var noSendChangeOutpoints []wdk.OutPoint
		for i := range noSendChainCount {
			t.Logf("Creating NoSend tx #%d", i+1)
			noSendChangeOutpoints, _ = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		}

		// and:
		// Call processAction using sendWith and IsNewTx set to false, including the two previous transactions in SendWithSlice.
		sendWithProcessAction := when.ProcessAction(wdk.ProcessActionArgs{
			IsNewTx:    false,
			IsNoSend:   false,
			SendWith:   when.NoSendTxsHexStrings(),
			IsSendWith: true,
		})

		// then:
		then.
			ProcessedSuccessfully(sendWithProcessAction).
			Funds().ShouldBeAbleToReserveSatoshis(
			inputSatoshis +
				-4 + // two no-send txs (2 sats each)
				-7, // fee to create a new transaction with many inputs
		)
	})

	t.Run("three no-send txs, single initial UTXO, all noSendChange is used making chain of txs, no newTx when sendWith", func(t *testing.T) {
		// given:
		const inputSatoshis = 6
		const noSendChainCount = 3

		givenStorage, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := givenStorage.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()
		given, when, then := nosendtest.New(t, testusers.Alice, givenStorage, activeStorage)

		// and:
		given.UserOwnsGivenUTXOsToSpend(inputSatoshis)

		// when:
		var noSendChangeOutpoints []wdk.OutPoint
		for i := range noSendChainCount {
			t.Logf("Creating NoSend tx #%d", i+1)
			noSendChangeOutpoints, _ = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		}

		// and:
		// Call processAction using sendWith and IsNewTx set to false, including the two previous transactions in SendWithSlice.
		sendWithProcessAction := when.ProcessAction(wdk.ProcessActionArgs{
			IsNewTx:    false,
			IsNoSend:   false,
			SendWith:   when.NoSendTxsHexStrings(),
			IsSendWith: true,
		})

		// then:
		then.
			ProcessedSuccessfully(sendWithProcessAction).
			Funds().ShouldNotBeAbleToReserveSatoshis(1) // All funds are tied up in the long NoSend chain
	})

	t.Run("two no-send txs, two initial UTXOs (one used for txs chain), not all noSendChange used, no newTx when sendWith", func(t *testing.T) {
		// NOTE: In this case, the balance after all should be made of one initial UTXO + not used noSendChanges + last no-send tx change
		// given:
		const initialUTXOSats = 10000
		const noSendChainCount = 2

		givenStorage, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := givenStorage.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()
		given, when, then := nosendtest.New(t, testusers.Alice, givenStorage, activeStorage)

		// and:
		given.UserOwnsGivenUTXOsToSpend(initialUTXOSats, initialUTXOSats)

		// when:
		var noSendChangeOutpoints []wdk.OutPoint
		for i := range noSendChainCount {
			t.Logf("Creating NoSend tx #%d", i+1)
			noSendChangeOutpoints, _ = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		}

		// and:
		// Call processAction using sendWith and IsNewTx set to false, including the two previous transactions in SendWithSlice.
		sendWithProcessAction := when.ProcessAction(wdk.ProcessActionArgs{
			IsNewTx:    false,
			IsNoSend:   false,
			SendWith:   when.NoSendTxsHexStrings(),
			IsSendWith: true,
		})

		// then:
		then.
			ProcessedSuccessfully(sendWithProcessAction).
			Funds().ShouldBeAbleToReserveSatoshis(initialUTXOSats - noSendChainCount*2)
	})

	t.Run("two no-send txs, many initial UTXOs, all noSendChange used, additional UTXO needed, no newTx when sendWith", func(t *testing.T) {
		// given:
		const inputSatoshis = 99904
		const largerUTXOToSend = 50_000

		givenStorage, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := givenStorage.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()
		given, when, then := nosendtest.New(t, testusers.Alice, givenStorage, activeStorage)

		// and:
		given.UserOwnsMultipleUTXOsToSpend(inputSatoshis)

		// when:
		// step 1:
		var noSendChangeOutpoints []wdk.OutPoint
		noSendChangeOutpoints, _ = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)

		// and:
		// step 2:
		when.WillSendSats(largerUTXOToSend)
		_, _ = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)

		// and:
		// Call processAction using sendWith and IsNewTx set to false, including the two previous transactions in SendWithSlice.
		sendWithProcessAction := when.ProcessAction(wdk.ProcessActionArgs{
			IsNewTx:    false,
			IsNoSend:   false,
			SendWith:   when.NoSendTxsHexStrings(),
			IsSendWith: true,
		})

		// then:
		then.
			ProcessedSuccessfully(sendWithProcessAction).
			Funds().ShouldBeAbleToReserveSatoshis(
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

		givenStorage, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := givenStorage.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()
		given, when, then := nosendtest.New(t, testusers.Alice, givenStorage, activeStorage)

		// and:
		given.UserOwnsMultipleUTXOsToSpend(inputSatoshis)

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
		var allocatedNoSendChangeOutpoints []wdk.OutPoint
		noSendChangeOutpoints, _ := when.CreateAndProcessNoSendAction(nil)
		require.Equal(t, 3, len(noSendChangeOutpoints), "there should be multiple nosend change outpoints")

		// and:
		// step 2:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		require.Equal(t, 1, len(noSendChangeOutpoints), "only one change output should be produced")
		require.Equal(t, 1, len(allocatedNoSendChangeOutpoints), "only one change output should be used")
		allRemainedNoSendChangeBeforeStep3 := len(when.AllRemainedNoSendChange())
		require.Equal(t, 3, allRemainedNoSendChangeBeforeStep3, "three no-send change outputs should remain")

		// and:
		// step 3:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints = when.
			WillSendSats(largerUTXOToSend).
			CreateAndProcessNoSendAction(when.AllRemainedNoSendChange())
		require.Equal(t, 2, len(noSendChangeOutpoints), "two change outputs should be produced")
		require.Equal(t, allRemainedNoSendChangeBeforeStep3, len(allocatedNoSendChangeOutpoints), "all previous no-send change outputs should be used")
		extraUTXOsOutOfNosendPool := len(when.LastCreateActionResult().Inputs) - len(allocatedNoSendChangeOutpoints)
		require.Equal(t, 15, extraUTXOsOutOfNosendPool, "funder should select at least one extra UTXO outside of no-send change outputs")

		// and:
		// Call processAction using sendWith and IsNewTx set to false, including the two previous transactions in SendWithSlice.
		sendWithProcessAction := when.ProcessAction(wdk.ProcessActionArgs{
			IsNewTx:    false,
			IsNoSend:   false,
			SendWith:   when.NoSendTxsHexStrings(),
			IsSendWith: true,
		})

		// then:
		then.
			ProcessedSuccessfully(sendWithProcessAction).
			Funds().ShouldBeAbleToReserveSatoshis(
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

	givenStorage, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := givenStorage.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()
	given, when, then := nosendtest.New(t, testusers.Alice, givenStorage, activeStorage)

	// and:
	given.UserOwnsMultipleUTXOsToSpend(inputSatoshis)

	// when:
	// step 1:
	noSendChangeOutpoints, _ := when.CreateAndProcessNoSendAction(nil)
	require.NotEmpty(t, noSendChangeOutpoints)

	// and:
	// step 2:
	noSendChangeOutpoints, _ = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)
	require.NotEmpty(t, noSendChangeOutpoints)

	// and:
	// Call createAction with NoSendChange AND SendWith
	// NOTE: While it is technically possible on storage level, this is not possible on wallet level,
	// because here we provide NoSend = true for createAction and NoSend = false for processAction
	thirdProcessActionResult, thirdTxID := when.CreateAndProcessSendWithAction(
		when.NoSendTxsHexStrings(),
		when.CreateActionNoSendArgsModifier(noSendChangeOutpoints, true),
		when.CreateActionSendWithArgsModifier(when.NoSendTxsHexStrings()...),
	)

	// then:
	then.ProcessedSuccessfully(thirdProcessActionResult, thirdTxID)
}

func TestNoSendSendWithScenario_SendWithSeparatedNewTx(t *testing.T) {
	const inputSatoshis = 99904

	givenStorage, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := givenStorage.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()
	given, when, then := nosendtest.New(t, testusers.Alice, givenStorage, activeStorage)

	// and:
	given.UserOwnsMultipleUTXOsToSpend(inputSatoshis)

	// when:
	// step 1:
	noSendChangeOutpoints, _ := when.CreateAndProcessNoSendAction(nil)
	require.NotEmpty(t, noSendChangeOutpoints)

	// and:
	// step 2:
	noSendChangeOutpoints, _ = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)
	require.NotEmpty(t, noSendChangeOutpoints)

	// and:
	thirdProcessActionResult, thirdTxID := when.CreateAndProcessSendWithAction(
		when.NoSendTxsHexStrings(),
		when.CreateActionNoSendArgsModifier(nil, false), // NOTE the nil as prevNoSendOutpoints, this makes this new tx out of chain of first-second NoSend txs
		when.CreateActionSendWithArgsModifier(when.NoSendTxsHexStrings()...),
	)

	// then:
	then.ProcessedWithServiceError(thirdProcessActionResult, thirdTxID)
}

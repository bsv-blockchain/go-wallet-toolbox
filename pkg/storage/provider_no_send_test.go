package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities/nosendtest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestNoSendPlusSendWithScenario(t *testing.T) {
	t.Run("two no-send txs, many initial UTXOs, all noSendChange is used, no newTx when sendWith", func(t *testing.T) {
		// given:
		const inputSatoshis = 99904

		given, when, then, cleanup := nosendtest.New(t, testusers.Alice)
		defer cleanup()

		// and:
		given.UserOwnsMultipleUTXOsToSpend(inputSatoshis)

		// when:
		// step 1:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints := when.CreateAndProcessNoSendAction(nil)
		assert.Empty(t, allocatedNoSendChangeOutpoints)
		assert.Len(t, noSendChangeOutpoints, 8)

		// and:
		// step 2:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		assert.Len(t, allocatedNoSendChangeOutpoints, 1)
		assert.Len(t, noSendChangeOutpoints, 2)

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

		given, when, then, cleanup := nosendtest.New(t, testusers.Alice)
		defer cleanup()

		// and:
		given.UserOwnsGivenUTXOsToSpend(inputSatoshis)

		// when:
		// step 1:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints := when.CreateAndProcessNoSendAction(nil)
		assert.Empty(t, allocatedNoSendChangeOutpoints)
		assert.Len(t, noSendChangeOutpoints, 1)

		// and:
		// step 2:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		assert.Len(t, allocatedNoSendChangeOutpoints, 1)
		assert.Len(t, noSendChangeOutpoints, 1)

		// and:
		// step 3:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		assert.Len(t, allocatedNoSendChangeOutpoints, 1)
		assert.Empty(t, noSendChangeOutpoints)

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

		given, when, then, cleanup := nosendtest.New(t, testusers.Alice)
		defer cleanup()

		// and:
		given.UserOwnsGivenUTXOsToSpend(initialUTXOSats, initialUTXOSats)

		// when:
		// step 1:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints := when.CreateAndProcessNoSendAction(nil)
		assert.Empty(t, allocatedNoSendChangeOutpoints)
		assert.Len(t, noSendChangeOutpoints, 8)

		// and:
		// step 2:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		assert.Len(t, allocatedNoSendChangeOutpoints, 1)
		assert.Len(t, noSendChangeOutpoints, 2)

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

		given, when, then, cleanup := nosendtest.New(t, testusers.Alice)
		defer cleanup()

		// and:
		given.UserOwnsMultipleUTXOsToSpend(inputSatoshis)

		// when:
		// step 1:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints := when.CreateAndProcessNoSendAction(nil)
		assert.Empty(t, allocatedNoSendChangeOutpoints)
		assert.Len(t, noSendChangeOutpoints, 8)

		// and:
		// step 2:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints = when.
			WillSendSats(largerUTXOToSend).
			CreateAndProcessNoSendAction(noSendChangeOutpoints)
		assert.Len(t, allocatedNoSendChangeOutpoints, 8)
		assert.Len(t, noSendChangeOutpoints, 8)

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

		given, when, then, cleanup := nosendtest.New(t, testusers.Alice)
		defer cleanup()

		// and:
		given.UserOwnsMultipleUTXOsToSpend(inputSatoshis)

		// NOTE: We need to reconfigure the basket to increase the number of UTXOs in the wallet
		// This way, createAction will produce more-than-one change outputs
		err := given.ActiveProvider().ConfigureBasket(t.Context(), testusers.Alice.AuthID(), wdk.BasketConfiguration{
			Name:                    wdk.BasketNameForChange,
			NumberOfDesiredUTXOs:    100,
			MinimumDesiredUTXOValue: 1000,
		})
		require.NoError(t, err)

		// when:
		// step 1:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints := when.CreateAndProcessNoSendAction(nil)
		assert.Empty(t, allocatedNoSendChangeOutpoints)
		assert.Len(t, noSendChangeOutpoints, 8)

		// and:
		// step 2:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints = when.CreateAndProcessNoSendAction(noSendChangeOutpoints)
		assert.Len(t, allocatedNoSendChangeOutpoints, 1)
		assert.Len(t, noSendChangeOutpoints, 2)
		allRemainedNoSendChangeBeforeStep3 := len(when.AllRemainedNoSendChange())
		assert.Equal(t, 9, allRemainedNoSendChangeBeforeStep3, "8-1+2=9 no-send change outputs should remain")

		// and:
		// step 3:
		noSendChangeOutpoints, allocatedNoSendChangeOutpoints = when.
			WillSendSats(largerUTXOToSend).
			CreateAndProcessNoSendAction(when.AllRemainedNoSendChange())
		assert.Len(t, allocatedNoSendChangeOutpoints, 9)
		assert.Len(t, noSendChangeOutpoints, 8)
		extraUTXOsOutOfNosendPool := len(when.LastCreateActionResult().Inputs) - len(allocatedNoSendChangeOutpoints)
		assert.Equal(t, 4, extraUTXOsOutOfNosendPool, "funder should select at least one extra UTXO outside of no-send change outputs")

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

	given, when, then, cleanup := nosendtest.New(t, testusers.Alice)
	defer cleanup()

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

	given, when, then, cleanup := nosendtest.New(t, testusers.Alice)
	defer cleanup()

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

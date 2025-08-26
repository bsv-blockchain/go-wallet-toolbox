package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestNoSendPlusSendWithScenario_SendWithoutNewTx(t *testing.T) {
	// given:
	const inputSatoshis = 99904

	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	givenNoSend := testabilities.GivenNoSend(t, given, activeStorage, testusers.Alice)

	// and:
	givenNoSend.FundWallet(inputSatoshis) // This makes the wallet not-empty with several UTXOs

	// when:
	noSendChangeOutpoints := givenNoSend.CreateAndProcessNoSendAction(nil)
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
	noSendChangeOutpoints := givenNoSend.CreateAndProcessNoSendAction(nil)
	noSendChangeOutpoints = givenNoSend.CreateAndProcessNoSendAction(noSendChangeOutpoints)

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
	noSendChangeOutpoints := givenNoSend.CreateAndProcessNoSendAction(nil)
	_ = givenNoSend.CreateAndProcessNoSendAction(noSendChangeOutpoints)

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

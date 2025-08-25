package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func TestNoSendSendWithScenario_SendWithoutNewTx(t *testing.T) {
	// given:
	const inputSatoshis = 99904

	fixture := testabilities.NewBuildNoSendTransactionFixture(t, inputSatoshis)
	defer fixture.Cleanup()

	result := fixture.BuildNoSendTransaction()

	// Step 3. Process the action using only the arguments, including the two previous txIDs, with IsNewTx set to false.
	thirdProcessActionResult := fixture.ProcessAction(testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:    false,
		IsNoSend:   false,
		SendWith:   []primitives.HexString{primitives.HexString(result.FirstTxID), primitives.HexString(result.SecondTxID)},
		IsSendWith: true,
	})

	testabilities.NotDelayedResultsAsserter(thirdProcessActionResult.NotDelayedResults).NotDelayedResultsContainTxsWithStatus(t, wdk.ReviewActionResultStatusSuccess,
		result.FirstTxID,
		result.SecondTxID,
	)

	testabilities.SendWithResultsAsseter(thirdProcessActionResult.SendWithResults).SendWithResultsContainTxsWithStatus(t, wdk.SendWithResultStatusUnproven,
		result.FirstTxID,
		result.SecondTxID,
	)

	testabilities.
		ThenDBState(t, fixture.ActiveStorage()).
		HasUserTransactionsByTxIDsWithStatus(testusers.Alice, wdk.TxStatusUnproven, result.FirstTxID, result.SecondTxID)
}

func TestNoSendSendWithScenario_SendWithNewTx(t *testing.T) {
	// given:
	const inputSatoshis = 99904

	fixture := testabilities.NewBuildNoSendTransactionFixture(t, inputSatoshis)
	defer fixture.Cleanup()

	result := fixture.BuildNoSendTransaction()

	// Step 3. Create an Action with NoSendChange using the result of the previous Create Action. Call CreateAction with IsNewTx set to true, including the two previous transactions in SendWithSlice.
	createActionArgs := fixtures.DefaultValidCreateActionArgs(func(args *wdk.ValidCreateActionArgs) {
		args.Outputs[0].Satoshis = 1
		args.IsNoSend = true
		args.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(true))
		args.Options.NoSendChange = slices.Map(result.SecondCreateActionResult.NoSendChangeOutputVouts, func(vout int) wdk.OutPoint {
			return wdk.OutPoint{
				TxID: result.SecondTxID,
				Vout: uint32(vout),
			}
		})
		args.IsNewTx = true
		args.IsSendWith = true
		args.Options.SendWith = []primitives.HexString{
			primitives.HexString(result.FirstTxID),
			primitives.HexString(result.SecondTxID),
		}
	})

	thirdCreateActionResult, thirdTx := fixture.CreateAction(createActionArgs)
	require.NotEmpty(t, result.FirstCreateActionResult.NoSendChangeOutputVouts)

	thirdTxID := thirdTx.TxID().String()

	thirdProcessActionResult := fixture.ProcessAction(fixture.UserAuthID(), wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  false,
		Reference: to.Ptr(thirdCreateActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(thirdTxID)),
		RawTx:     thirdTx.Bytes(),
		SendWith: []primitives.HexString{
			primitives.HexString(result.FirstTxID),
			primitives.HexString(result.SecondTxID),
		},
		IsSendWith: true,
	})

	testabilities.NotDelayedResultsAsserter(thirdProcessActionResult.NotDelayedResults).NotDelayedResultsContainTxsWithStatus(t, wdk.ReviewActionResultStatusSuccess,
		result.FirstTxID,
		result.SecondTxID,
		thirdTxID,
	)

	testabilities.SendWithResultsAsseter(thirdProcessActionResult.SendWithResults).SendWithResultsContainTxsWithStatus(t, wdk.SendWithResultStatusUnproven,
		result.FirstTxID,
		result.SecondTxID,
		thirdTxID,
	)

	testabilities.
		ThenDBState(t, fixture.ActiveStorage()).
		HasUserTransactionsByTxIDsWithStatus(testusers.Alice, wdk.TxStatusUnproven, result.FirstTxID, result.SecondTxID, thirdTxID)
}

func TestNoSendSendWithScenario_SendWithSeparatedNewTx(t *testing.T) {
	// given:
	const inputSatoshis = 99904

	fixture := testabilities.NewBuildNoSendTransactionFixture(t, inputSatoshis)
	defer fixture.Cleanup()

	result := fixture.BuildNoSendTransaction()

	// Step 3. Create an Action without NoSendChange from the previous action result, UTXOs will originate from different transactions.
	createActionArgs := fixtures.DefaultValidCreateActionArgs(func(args *wdk.ValidCreateActionArgs) {
		args.Outputs[0].Satoshis = 1
		args.Options.NoSendChange = nil
		args.IsNewTx = true
		args.IsSendWith = true
		args.Options.SendWith = []primitives.HexString{
			primitives.HexString(result.FirstTxID),
			primitives.HexString(result.SecondTxID),
		}
	})

	thirdCreateActionResult, thirdTx := fixture.CreateAction(createActionArgs)
	thirdTxID := thirdTx.TxID().String()

	thirdProcessActionResult := fixture.ProcessAction(testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  false,
		Reference: to.Ptr(thirdCreateActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(thirdTxID)),
		RawTx:     thirdTx.Bytes(),
		SendWith: []primitives.HexString{
			primitives.HexString(result.FirstTxID),
			primitives.HexString(result.SecondTxID),
		},
		IsSendWith: true,
	})

	// NOTE: ServiceError is because only working broadcaster for unit tests is ARC which does not support sending BEEFs that have multiple subject transactions
	testabilities.NotDelayedResultsAsserter(thirdProcessActionResult.NotDelayedResults).NotDelayedResultsContainTxsWithStatus(t, wdk.ReviewActionResultStatusServiceError,
		result.FirstTxID,
		result.SecondTxID,
		thirdTxID,
	)

	testabilities.SendWithResultsAsseter(thirdProcessActionResult.SendWithResults).SendWithResultsContainTxsWithStatus(t, wdk.SendWithResultStatusSending,
		result.FirstTxID,
		result.SecondTxID,
		thirdTxID,
	)

	testabilities.
		ThenDBState(t, fixture.ActiveStorage()).
		HasUserTransactionsByTxIDsWithStatus(testusers.Alice, wdk.TxStatusSending, result.FirstTxID, result.SecondTxID, thirdTxID)
}

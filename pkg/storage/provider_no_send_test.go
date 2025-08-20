package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func TestProcessAction_SendFlagFalse_SendWithSlice_IncludesTwoTransactions(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	const inputSatoshish = 99904

	keyDeriver := testusers.Alice.KeyDeriver(t)
	activeStorage := given.Provider().GORM()

	given.
		Action(activeStorage).
		WithSatoshisToInternalize(inputSatoshish).
		WithSatoshisToSend(1).
		Processed()

	// Step 1 - Process Action with IsNoSend flag set to true, empty noSendChange outpoints. Inputs will be allocated
	// normally from spendable outputs (basket for change)
	createActionArgs := fixtures.DefaultValidCreateActionArgs()
	createActionArgs.Outputs[0].Satoshis = 1
	createActionArgs.IsNoSend = true
	createActionArgs.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(true))

	firstCreateActionResult, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), createActionArgs)
	require.NoError(t, err)
	require.NotNil(t, firstCreateActionResult)
	require.NotEmpty(t, firstCreateActionResult.NoSendChangeOutputVouts)

	firstTx, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, firstCreateActionResult).Assemble()
	require.NoError(t, err)
	require.NotNil(t, firstTx)
	require.NoError(t, firstTx.Sign())

	firstTxID := firstTx.TxID().String()

	firstProcessActionArgs := wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true,
		Reference: to.Ptr(firstCreateActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(firstTxID)),
		RawTx:     firstTx.Bytes(),
	}

	firstProcessActionResult, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), firstProcessActionArgs)
	require.NoError(t, err)
	require.NotNil(t, firstProcessActionResult)

	// Step 2 - Create Action With NoSendChange from the previous create action result.
	createActionArgs.Options.NoSendChange = slices.Map(firstCreateActionResult.NoSendChangeOutputVouts, func(vout int) wdk.OutPoint {
		return wdk.OutPoint{TxID: firstTxID, Vout: uint32(vout)}
	})

	secondCreateActionResult, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), createActionArgs)
	require.NoError(t, err)
	require.NotNil(t, secondCreateActionResult)

	secondTx, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, secondCreateActionResult).Assemble()
	require.NoError(t, err)
	require.NotNil(t, secondTx)
	require.NoError(t, secondTx.Sign())

	secondTxID := secondTx.TxID().String()

	secondProcessActionArgs := wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true,
		Reference: to.Ptr(secondCreateActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(secondTxID)),
		RawTx:     secondTx.Bytes(),
	}

	secondProcessActionResult, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), secondProcessActionArgs)
	require.NoError(t, err)
	require.NotNil(t, secondProcessActionResult)

	// Step 3 - Process Action only with args including two previous txIDs and IsNewTx false
	thirdProcessActionArgs := wdk.ProcessActionArgs{
		IsNewTx:    false,
		IsNoSend:   false,
		SendWith:   []primitives.HexString{primitives.HexString(firstTxID), primitives.HexString(secondTxID)},
		IsSendWith: true,
	}

	thirdProcessActionResult, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), thirdProcessActionArgs)
	require.NoError(t, err)
	require.NotNil(t, thirdProcessActionResult)

	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, firstTxID).WithStatus(wdk.TxStatusUnproven)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, secondTxID).WithStatus(wdk.TxStatusUnproven)
}

func TestProcessAction_NegativePath(t *testing.T) { // TODO: Temp name for the test.
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	const inputSatoshish = 99904

	keyDeriver := testusers.Alice.KeyDeriver(t)
	activeStorage := given.Provider().GORM()

	given.
		Action(activeStorage).
		WithSatoshisToInternalize(inputSatoshish).
		WithSatoshisToSend(1).
		Processed()

	// Step 1 - Process Action with IsNoSend flag set to true, empty noSendChange outpoints. Inputs will be allocated
	// normally from spendable outputs (basket for change)
	createActionArgs := fixtures.DefaultValidCreateActionArgs()
	createActionArgs.Outputs[0].Satoshis = 1
	createActionArgs.IsNoSend = true
	createActionArgs.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(true))

	firstCreateActionResult, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), createActionArgs)
	require.NoError(t, err)
	require.NotNil(t, firstCreateActionResult)
	require.NotEmpty(t, firstCreateActionResult.NoSendChangeOutputVouts)

	firstTx, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, firstCreateActionResult).Assemble()
	require.NoError(t, err)
	require.NotNil(t, firstTx)
	require.NoError(t, firstTx.Sign())

	firstTxID := firstTx.TxID().String()

	firstProcessActionArgs := wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true,
		Reference: to.Ptr(firstCreateActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(firstTxID)),
		RawTx:     firstTx.Bytes(),
	}

	firstProcessActionResult, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), firstProcessActionArgs)
	require.NoError(t, err)
	require.NotNil(t, firstProcessActionResult)

	// Step 2 - Create Action With NoSendChange from the previous create action result.
	createActionArgs.Options.NoSendChange = slices.Map(firstCreateActionResult.NoSendChangeOutputVouts, func(vout int) wdk.OutPoint {
		return wdk.OutPoint{TxID: firstTxID, Vout: uint32(vout)}
	})

	secondCreateActionResult, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), createActionArgs)
	require.NoError(t, err)
	require.NotNil(t, secondCreateActionResult)

	secondTx, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, secondCreateActionResult).Assemble()
	require.NoError(t, err)
	require.NotNil(t, secondTx)
	require.NoError(t, secondTx.Sign())

	secondTxID := secondTx.TxID().String()
	secondProcessActionArgs := wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true,
		Reference: to.Ptr(secondCreateActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(secondTxID)),
		RawTx:     secondTx.Bytes(),
	}

	secondProcessActionResult, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), secondProcessActionArgs)
	require.NoError(t, err)
	require.NotNil(t, secondProcessActionResult)

	// Step 3
	createActionArgs.Options.NoSendChange = slices.Map(secondCreateActionResult.NoSendChangeOutputVouts, func(vout int) wdk.OutPoint {
		return wdk.OutPoint{
			TxID: secondTxID,
			Vout: uint32(vout),
		}
	})

	createActionArgs.IsNewTx = true
	createActionArgs.IsSendWith = true
	createActionArgs.Options.SendWith = []primitives.HexString{
		primitives.HexString(firstTxID),
		primitives.HexString(secondTxID),
	}

	thirdCreateActionResult, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), createActionArgs)
	require.NoError(t, err)
	require.NotNil(t, thirdCreateActionResult)

	thirdTx, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, secondCreateActionResult).Assemble()
	require.NoError(t, err)
	require.NotNil(t, thirdTx)
	require.NoError(t, thirdTx.Sign())

	thirdTxID := thirdTx.TxID().String()

	thirdProcessActionArgs := wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  false,
		Reference: to.Ptr(thirdCreateActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(thirdTxID)),
		RawTx:     thirdTx.Bytes(),
		SendWith: []primitives.HexString{
			primitives.HexString(firstTxID),
			primitives.HexString(secondTxID),
		},
		IsSendWith: true,
	}

	thirdProcessActionResult, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), thirdProcessActionArgs)
	require.NoError(t, err)
	require.NotNil(t, thirdProcessActionResult)

	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, firstTxID).WithStatus(wdk.TxStatusUnproven)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, secondTxID).WithStatus(wdk.TxStatusUnproven)
}

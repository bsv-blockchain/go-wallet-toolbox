package storage_test

import (
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNoSendWithSendWith(t *testing.T) {
	//testmode.DevelopmentOnly_SetFileSQLiteMode(t)
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	keyDeriver := givenKeyDeriver(t, testusers.Alice)

	const inputSatoshis = 99904

	given.Action(activeStorage).
		WithSatoshisToInternalize(inputSatoshis).
		WithSatoshisToSend(1).
		Processed()

	// 1
	args := fixtures.DefaultValidCreateActionArgs()
	args.Outputs[0].Satoshis = 1
	args.IsNoSend = true
	args.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(true))

	createActionResult, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)
	require.NoError(t, err)

	signed, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, createActionResult).Assemble()
	require.NoError(t, err)
	require.NotNil(t, signed)

	err = signed.Sign()
	require.NoError(t, err)

	firstTxID := signed.TxID().String()

	processActionArgs := wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true,
		Reference: to.Ptr(createActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(signed.TxID().String())),
		RawTx:     signed.Bytes(),
	}

	processActionResult, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), processActionArgs)
	require.NoError(t, err)
	require.NotNil(t, processActionResult)

	require.NotEmpty(t, createActionResult.NoSendChangeOutputVouts)

	// 2
	args.Options.NoSendChange = slices.Map(createActionResult.NoSendChangeOutputVouts, func(vout int) wdk.OutPoint {
		return wdk.OutPoint{
			TxID: firstTxID,
			Vout: uint32(vout),
		}
	})

	createActionResult, err = activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)
	require.NoError(t, err)

	signed, err = assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, createActionResult).Assemble()
	require.NoError(t, err)
	require.NotNil(t, signed)

	err = signed.Sign()
	require.NoError(t, err)

	processActionArgs = wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true,
		Reference: to.Ptr(createActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(signed.TxID().String())),
		RawTx:     signed.Bytes(),
	}

	processActionResult, err = activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), processActionArgs)
	require.NoError(t, err)
	require.NotNil(t, processActionResult)

	require.NotEmpty(t, createActionResult.NoSendChangeOutputVouts)

	// 3
	secondTxID := signed.TxID().String()

	processActionArgs = wdk.ProcessActionArgs{
		IsNewTx:  false,
		IsNoSend: false,
		SendWith: []primitives.TXIDHexString{
			primitives.TXIDHexString(firstTxID),
			primitives.TXIDHexString(secondTxID),
		},
		IsSendWith: true,
	}

	processActionResult, err = activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), processActionArgs)
	require.NoError(t, err)
	require.NotNil(t, processActionResult)

	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, firstTxID).WithStatus(wdk.TxStatusUnproven)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, secondTxID).WithStatus(wdk.TxStatusUnproven)
}

func givenKeyDeriver(t *testing.T, user testusers.User) *sdk.KeyDeriver {
	priv, err := ec.PrivateKeyFromHex(user.PrivKey)
	require.NoError(t, err)

	return sdk.NewKeyDeriver(priv)
}

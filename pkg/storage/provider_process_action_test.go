package storage_test

import (
	"context"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/randomizer"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessActionHappyPath(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.ActionCreatedAndSigned(activeStorage)
	txID := signedTx.TxID().String()

	// and:
	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []string{},
	}

	// when:
	result, err := activeStorage.ProcessAction(context.Background(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusUnproven, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 1)
	reviewActionResult := result.NotDelayedResults[0]
	assert.Equal(t, txID, string(reviewActionResult.TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusSuccess, reviewActionResult.Status)
	assert.Empty(t, reviewActionResult.CompetingTxs)
}

func TestProcessActionTwice(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.ActionCreatedAndSigned(activeStorage)
	txID := signedTx.TxID().String()

	// and:
	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []string{},
	}

	// when:
	_, err := activeStorage.ProcessAction(context.Background(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	// when:
	args.IsNewTx = false
	result, err := activeStorage.ProcessAction(context.Background(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusUnproven, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 0)
}

func TestProcessActionErrorCases(t *testing.T) {
	tests := map[string]struct {
		argsModifier func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs
	}{
		"IsNewTx set to false for not stored tx": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.IsNewTx = false
				return args
			},
		},
		"not existing reference": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.Reference = to.Ptr("not-existing-reference")
				return args
			},
		},
		"tx id missmatch": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				otherID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID()
				args.TxID = to.Ptr(primitives.TXIDHexString(otherID))
				return args
			},
		},
		"empty raw tx": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.RawTx = []byte{}
				return args
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()
			activeStorage := given.Provider().
				WithRandomizer(randomizer.NewTestRandomizer()).
				GORM()

			// and:
			createActionResult, signedTx := given.ActionCreatedAndSigned(activeStorage)
			txID := signedTx.TxID().String()

			// and:
			args := test.argsModifier(wdk.ProcessActionArgs{
				IsNewTx:    false,
				IsSendWith: false,
				IsNoSend:   false,
				IsDelayed:  false,
				Reference:  to.Ptr(createActionResult.Reference),
				TxID:       to.Ptr(primitives.TXIDHexString(txID)),
				RawTx:      signedTx.Bytes(),
				SendWith:   []string{},
			})

			// when:
			_, err := activeStorage.ProcessAction(context.Background(), testusers.Alice.AuthID(), args)

			// then:
			require.Error(t, err)
		})
	}
}

func TestProcessActionDoubleSpending(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.ActionCreatedAndSigned(activeStorage)
	txID := signedTx.TxID().String()

	// and:
	otherTXID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID()
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnDoubleSpending(otherTXID)

	// and:
	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []string{},
	}

	// when:
	result, err := activeStorage.ProcessAction(context.Background(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusFailed, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 1)
	reviewActionResult := result.NotDelayedResults[0]
	assert.Equal(t, txID, string(reviewActionResult.TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusDoubleSpend, reviewActionResult.Status)

	require.Len(t, reviewActionResult.CompetingTxs, 1)
	assert.Equal(t, otherTXID, reviewActionResult.CompetingTxs[0])
}

func TestProcessActionARCReturnNoBody(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.ActionCreatedAndSigned(activeStorage)
	txID := signedTx.TxID().String()

	// and:
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnNoBody()

	// and:
	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []string{},
	}

	// when:
	result, err := activeStorage.ProcessAction(context.Background(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusSending, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 1)
	reviewActionResult := result.NotDelayedResults[0]
	assert.Equal(t, txID, string(reviewActionResult.TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusServiceError, reviewActionResult.Status)
	assert.Empty(t, reviewActionResult.CompetingTxs)
}

package methodtests

import (
	"context"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/randomizer"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func TestProcessActionHappyPath(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.ActionCreated(activeStorage)
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

	// TODO: Check result after broadcasting
}

func TestProcessActionTwice(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.ActionCreated(activeStorage)
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
	_, err = activeStorage.ProcessAction(context.Background(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)
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
			given := testabilities.Given(t)
			activeStorage := given.Provider().
				WithRandomizer(randomizer.NewTestRandomizer()).
				GORM()

			// and:
			createActionResult, signedTx := given.ActionCreated(activeStorage)
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

package fixtures

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

const DefaultCreateActionOutputSatoshis = 42000

func DefaultWalletCreateActionArgs(t *testing.T, opts ...func(*sdk.CreateActionArgs)) sdk.CreateActionArgs {
	t.Helper()

	lockingScript, err := script.NewFromHex("76a914dbc0a7c84983c5bf199b7b2d41b3acf0408ee5aa88ac")
	require.NoError(t, err, "Failed to decode locking script: INVALID TEST SETUP")

	args := to.OptionsWithDefault(sdk.CreateActionArgs{
		Description: "test transaction",
		InputBEEF:   nil,
		Inputs:      nil,
		Outputs: []sdk.CreateActionOutput{
			{
				LockingScript:      lockingScript.Bytes(),
				Satoshis:           DefaultCreateActionOutputSatoshis,
				OutputDescription:  "test output",
				CustomInstructions: CreateActionTestCustomInstructions,
				Tags:               []string{CreateActionTestTag},
			},
		},
		LockTime: WalletLockTime,
		Version:  WalletTxVersion,
		Labels:   []string{CreateActionTestLabel},
		Options: &sdk.CreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr(false),
			SignAndProcess:         to.Ptr(true),
			RandomizeOutputs:       to.Ptr(false),
		},
	}, opts...)

	return args
}

func DefaultValidCreateActionArgs(opts ...func(*wdk.ValidCreateActionArgs)) wdk.ValidCreateActionArgs {
	return to.OptionsWithDefault(wdk.ValidCreateActionArgs{
		Description: "test transaction",
		InputBEEF:   nil,
		Inputs:      []wdk.ValidCreateActionInput{},
		Outputs: []wdk.ValidCreateActionOutput{
			{
				LockingScript:      "76a914dbc0a7c84983c5bf199b7b2d41b3acf0408ee5aa88ac",
				Satoshis:           DefaultCreateActionOutputSatoshis,
				OutputDescription:  "test output",
				CustomInstructions: to.Ptr(CreateActionTestCustomInstructions),
				Tags:               []primitives.StringUnder300{CreateActionTestTag},
			},
		},
		LockTime: 0,
		Version:  1,
		Labels:   []primitives.StringUnder300{CreateActionTestLabel},
		Options: wdk.ValidCreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr[primitives.BooleanDefaultTrue](false),
			SendWith:               []primitives.TXIDHexString{},
			SignAndProcess:         to.Ptr(primitives.BooleanDefaultTrue(true)),
			KnownTxids:             []primitives.TXIDHexString{},
			NoSendChange:           []wdk.OutPoint{},
			RandomizeOutputs:       false,
			TrustSelf:              nil,
		},
		IsSendWith:                   false,
		IsDelayed:                    false,
		IsNoSend:                     false,
		IsNewTx:                      true,
		IsRemixChange:                false,
		IsSignAction:                 false,
		IncludeAllSourceTransactions: true,
	}, opts...)
}

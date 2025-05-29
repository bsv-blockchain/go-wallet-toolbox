package fixtures

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
)

func DefaultValidCreateActionArgs() wdk.ValidCreateActionArgs {
	return wdk.ValidCreateActionArgs{
		Description: "outputBRC29",
		InputBEEF:   nil,
		Inputs:      []wdk.ValidCreateActionInput{},
		Outputs: []wdk.ValidCreateActionOutput{
			{
				LockingScript:      "76a914dbc0a7c84983c5bf199b7b2d41b3acf0408ee5aa88ac",
				Satoshis:           42000,
				OutputDescription:  "outputBRC29",
				CustomInstructions: to.Ptr(`{"derivationPrefix":"bPRI9FYwsIo=","derivationSuffix":"FdjLdpnLnJM=","type":"BRC29"}`),
			},
		},
		LockTime: 0,
		Version:  1,
		Labels:   []primitives.StringUnder300{"outputbrc29"},
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
	}
}

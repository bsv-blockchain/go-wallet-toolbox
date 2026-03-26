package validate

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestForDefaultValidCreateActionArgs(t *testing.T) {
	// given:
	args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)

	// when:
	err := ValidInternalizeActionArgs(&args)

	// then:
	require.NoError(t, err)
}

func TestWrongInternalizeActionArgs(t *testing.T) {
	tests := map[string]struct {
		modifier func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs
	}{
		"Tx empty": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Tx = []byte{}
				return args
			},
		},
		"Outputs empty": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Outputs = []*wdk.InternalizeOutput{}
				return args
			},
		},
		"Description too short": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Description = "sh"
				return args
			},
		},
		"Label too long": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Labels = []primitives.StringUnder300{primitives.StringUnder300(bytes.Repeat([]byte{'a'}, 301))}
				return args
			},
		},
		"Output empty protocol": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				// assume at least one output exists in the default args
				args.Outputs[0].Protocol = ""
				return args
			},
		},
		"Output WalletPayment missing PaymentRemittance": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Outputs[0].Protocol = wdk.WalletPaymentProtocol
				args.Outputs[0].PaymentRemittance = nil
				return args
			},
		},
		"Output WalletPayment invalid: wrong derivationPrefix": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Outputs[0].Protocol = wdk.WalletPaymentProtocol
				args.Outputs[0].PaymentRemittance.DerivationPrefix = "not-a-base64-hex"
				return args
			},
		},
		"Output WalletPayment invalid: wrong derivationSuffix": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Outputs[0].Protocol = wdk.WalletPaymentProtocol
				args.Outputs[0].PaymentRemittance.DerivationSuffix = "not-a-base64-hex"
				return args
			},
		},
		"Output BasketInsertion missing InsertionRemittance": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Outputs[0].Protocol = wdk.BasketInsertionProtocol
				args.Outputs[0].InsertionRemittance = nil
				return args
			},
		},
		"Output BasketInsertion invalid: empty basket": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Outputs[0].Protocol = wdk.BasketInsertionProtocol
				if args.Outputs[0].InsertionRemittance != nil {
					args.Outputs[0].InsertionRemittance.Basket = ""
				}
				return args
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			defaultArgs := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
			modifiedArgs := test.modifier(defaultArgs)
			err := ValidInternalizeActionArgs(&modifiedArgs)
			require.Error(t, err)
		})
	}
}

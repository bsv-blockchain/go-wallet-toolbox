package validate_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestForDefaultProcessActionArgs(t *testing.T) {
	// given:
	args := fixtures.DefaultProcessActionArgs(t)

	// when:
	err := validate.ProcessActionArgs(&args)

	// then:
	require.NoError(t, err)
}

func TestWrongProcessActionArgs(t *testing.T) {
	tests := map[string]struct {
		modifier func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs
	}{
		"TxID invalid": {
			modifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.TxID = to.Ptr[primitives.TXIDHexString]("invalid")
				return args
			},
		},
		"NewTx missing reference": {
			modifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.IsNewTx = true
				args.Reference = nil
				return args
			},
		},
		"NewTx missing rawTx": {
			modifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.IsNewTx = true
				args.RawTx = nil
				return args
			},
		},
		"NewTx missing txID": {
			modifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.IsNewTx = true
				args.TxID = nil
				return args
			},
		},
		"IsSendWith true but no sendWith arguments": {
			modifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.IsSendWith = true
				args.SendWith = nil
				return args
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			defaultArgs := fixtures.DefaultProcessActionArgs(t)

			err := validate.ProcessActionArgs(to.Ptr(test.modifier(defaultArgs)))

			require.Error(t, err)
		})
	}
}

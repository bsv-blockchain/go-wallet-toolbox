package validate_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestForDefaultValidRelinquishCertificateArgs(t *testing.T) {
	// given:
	args := fixtures.DefaultValidRelinquishCertificateArgs()

	// when:
	err := validate.RelinquishCertificateArgs(args)

	// then:
	require.NoError(t, err)
}

func TestWrongRelinquishCertificateArgs(t *testing.T) {
	tests := map[string]struct {
		modifier func(args *wdk.RelinquishCertificateArgs) *wdk.RelinquishCertificateArgs
	}{
		"Invalid Type (non-base64 characters)": {
			modifier: func(args *wdk.RelinquishCertificateArgs) *wdk.RelinquishCertificateArgs {
				args.Type = "invalid!base64@"
				return args
			},
		},
		"Type with incorrect padding": {
			modifier: func(args *wdk.RelinquishCertificateArgs) *wdk.RelinquishCertificateArgs {
				args.Type = "abcd===" // Invalid padding for base64
				return args
			},
		},
		"Invalid SerialNumber (non-base64)": {
			modifier: func(args *wdk.RelinquishCertificateArgs) *wdk.RelinquishCertificateArgs {
				args.SerialNumber = "serial@number!"
				return args
			},
		},
		"SerialNumber with URL-unsafe characters": {
			modifier: func(args *wdk.RelinquishCertificateArgs) *wdk.RelinquishCertificateArgs {
				args.SerialNumber = "++//==" // Valid base64 but check if your impl allows it
				return args
			},
		},
		"Invalid Certifier (non-hex characters)": {
			modifier: func(args *wdk.RelinquishCertificateArgs) *wdk.RelinquishCertificateArgs {
				args.Certifier = "ghijk!"
				return args
			},
		},
		"Certifier with odd length": {
			modifier: func(args *wdk.RelinquishCertificateArgs) *wdk.RelinquishCertificateArgs {
				args.Certifier = "abc" // Odd length hex
				return args
			},
		},
		"Empty Certifier": {
			modifier: func(args *wdk.RelinquishCertificateArgs) *wdk.RelinquishCertificateArgs {
				args.Certifier = ""
				return args
			},
		},
		"All fields invalid simultaneously": {
			modifier: func(args *wdk.RelinquishCertificateArgs) *wdk.RelinquishCertificateArgs {
				args.Type = "!"
				args.SerialNumber = "@"
				args.Certifier = "!"
				return args
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			defaultArgs := fixtures.DefaultValidRelinquishCertificateArgs()
			modifiedArgs := test.modifier(defaultArgs)

			// when:
			err := validate.RelinquishCertificateArgs(modifiedArgs)

			// then:
			require.Error(t, err)
		})
	}
}

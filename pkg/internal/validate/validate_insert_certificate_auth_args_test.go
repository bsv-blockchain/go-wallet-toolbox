package validate_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestForDefaultValidInsertCertificateAuthArgs(t *testing.T) {
	// given:
	args := fixtures.DefaultInsertCertAuth(1, fixtures.SubjectPubKey)

	// when:
	err := validate.TableCertificateX(args)

	// then:
	require.NoError(t, err)
}

func TestWrongInsertCertificateAuthArgs(t *testing.T) {
	tests := map[string]struct {
		modifier func(args *wdk.TableCertificateX) *wdk.TableCertificateX
	}{
		"Invalid Type (non-hex characters)": {
			modifier: func(args *wdk.TableCertificateX) *wdk.TableCertificateX {
				args.Type = "ghijk!" // Contains invalid hex character '!'
				return args
			},
		},
		"Invalid Type (odd length)": {
			modifier: func(args *wdk.TableCertificateX) *wdk.TableCertificateX {
				args.Type = "abc" // Odd length
				return args
			},
		},
		"Invalid SerialNumber (wrong base64)": {
			modifier: func(args *wdk.TableCertificateX) *wdk.TableCertificateX {
				args.SerialNumber = "ZXhhbXBsZVR5cGUy==="
				return args
			},
		},
		"Invalid Certifier (too long)": {
			modifier: func(args *wdk.TableCertificateX) *wdk.TableCertificateX {
				args.Certifier = primitives.PubKeyHex(strings.Repeat("a", 301))
				return args
			},
		},
		"Invalid Subject (empty)": {
			modifier: func(args *wdk.TableCertificateX) *wdk.TableCertificateX {
				args.Subject = ""
				return args
			},
		},
		"Invalid Verifier (non-hex)": {
			modifier: func(args *wdk.TableCertificateX) *wdk.TableCertificateX {
				invalid := primitives.PubKeyHex("zzzz")
				args.Verifier = &invalid
				return args
			},
		},
		"Invalid RevocationOutpoint (missing index)": {
			modifier: func(args *wdk.TableCertificateX) *wdk.TableCertificateX {
				args.RevocationOutpoint = "txidwithoutindex"
				return args
			},
		},
		"Invalid Signature (odd length)": {
			modifier: func(args *wdk.TableCertificateX) *wdk.TableCertificateX {
				args.Signature = primitives.HexString("abc")
				return args
			},
		},
		"Field with invalid MasterKey (non-hex)": {
			modifier: func(args *wdk.TableCertificateX) *wdk.TableCertificateX {
				args.Fields[0].MasterKey = "invalidhex"
				return args
			},
		},
		"Invalid RevocationOutpoint index": {
			modifier: func(args *wdk.TableCertificateX) *wdk.TableCertificateX {
				args.RevocationOutpoint = "deadbeef.invalid"
				return args
			},
		},
		"Verifier with odd length": {
			modifier: func(args *wdk.TableCertificateX) *wdk.TableCertificateX {
				oddLength := primitives.PubKeyHex("abc")
				args.Verifier = &oddLength
				return args
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			defaultArgs := fixtures.DefaultInsertCertAuth(1, fixtures.SubjectPubKey)
			modifiedArgs := test.modifier(defaultArgs)

			// when:
			err := validate.TableCertificateX(modifiedArgs)

			// then:
			require.Error(t, err)
		})
	}
}

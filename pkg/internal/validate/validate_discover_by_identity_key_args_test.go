package validate_test

import (
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
)

func validIdentityKey(t *testing.T) *ec.PublicKey {
	t.Helper()
	privKey, err := ec.NewPrivateKey()
	require.NoError(t, err)
	return privKey.PubKey()
}

func TestDiscoverByIdentityKeyArgs_Success(t *testing.T) {
	tests := map[string]func(t *testing.T) sdk.DiscoverByIdentityKeyArgs{
		"valid args with identity key only": func(t *testing.T) sdk.DiscoverByIdentityKeyArgs {
			return sdk.DiscoverByIdentityKeyArgs{
				IdentityKey: validIdentityKey(t),
			}
		},
		"valid args with limit": func(t *testing.T) sdk.DiscoverByIdentityKeyArgs {
			return sdk.DiscoverByIdentityKeyArgs{
				IdentityKey: validIdentityKey(t),
				Limit:       to.Ptr(uint32(100)),
			}
		},
		"valid args with offset": func(t *testing.T) sdk.DiscoverByIdentityKeyArgs {
			return sdk.DiscoverByIdentityKeyArgs{
				IdentityKey: validIdentityKey(t),
				Offset:      to.Ptr(uint32(50)),
			}
		},
		"valid args with max limit": func(t *testing.T) sdk.DiscoverByIdentityKeyArgs {
			return sdk.DiscoverByIdentityKeyArgs{
				IdentityKey: validIdentityKey(t),
				Limit:       to.Ptr(uint32(validate.MaxPaginationLimit)),
			}
		},
		"valid args with max offset": func(t *testing.T) sdk.DiscoverByIdentityKeyArgs {
			return sdk.DiscoverByIdentityKeyArgs{
				IdentityKey: validIdentityKey(t),
				Offset:      to.Ptr(uint32(validate.MaxPaginationOffset)),
			}
		},
		"valid args with limit and offset": func(t *testing.T) sdk.DiscoverByIdentityKeyArgs {
			return sdk.DiscoverByIdentityKeyArgs{
				IdentityKey: validIdentityKey(t),
				Limit:       to.Ptr(uint32(50)),
				Offset:      to.Ptr(uint32(100)),
			}
		},
	}

	for name, argsFunc := range tests {
		t.Run(name, func(t *testing.T) {
			args := argsFunc(t)
			err := validate.DiscoverByIdentityKeyArgs(args)
			require.NoError(t, err)
		})
	}
}

func TestDiscoverByIdentityKeyArgs_Error(t *testing.T) {
	tests := map[string]struct {
		args        func(t *testing.T) sdk.DiscoverByIdentityKeyArgs
		expectedErr string
	}{
		"nil identity key": {
			args: func(t *testing.T) sdk.DiscoverByIdentityKeyArgs {
				return sdk.DiscoverByIdentityKeyArgs{
					IdentityKey: nil,
				}
			},
			expectedErr: "identityKey is required",
		},
		"limit below minimum": {
			args: func(t *testing.T) sdk.DiscoverByIdentityKeyArgs {
				return sdk.DiscoverByIdentityKeyArgs{
					IdentityKey: validIdentityKey(t),
					Limit:       to.Ptr(uint32(0)),
				}
			},
			expectedErr: "limit must be greater than 0",
		},
		"limit exceeds maximum": {
			args: func(t *testing.T) sdk.DiscoverByIdentityKeyArgs {
				return sdk.DiscoverByIdentityKeyArgs{
					IdentityKey: validIdentityKey(t),
					Limit:       to.Ptr(uint32(validate.MaxPaginationLimit + 1)),
				}
			},
			expectedErr: "limit exceeds max allowed value",
		},
		"offset exceeds maximum": {
			args: func(t *testing.T) sdk.DiscoverByIdentityKeyArgs {
				return sdk.DiscoverByIdentityKeyArgs{
					IdentityKey: validIdentityKey(t),
					Offset:      to.Ptr(uint32(validate.MaxPaginationOffset + 1)),
				}
			},
			expectedErr: "offset is too large",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			args := tc.args(t)
			err := validate.DiscoverByIdentityKeyArgs(args)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

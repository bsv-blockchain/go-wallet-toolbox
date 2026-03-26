package fixtures

import (
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func DefaultInternalizeActionArgs(t *testing.T, protocol wdk.InternalizeProtocol) wdk.InternalizeActionArgs {
	t.Helper()

	spec := testabilities.GivenTX().WithInput(1000).WithP2PKHOutput(ExpectedValueToInternalize)

	atomicBeef, err := spec.TX().AtomicBEEF(false)
	require.NoError(t, err)

	outputSpec := &wdk.InternalizeOutput{
		OutputIndex: 0,
		Protocol:    protocol,
	}
	if protocol == wdk.WalletPaymentProtocol {
		outputSpec.PaymentRemittance = &wdk.WalletPayment{
			DerivationPrefix:  DerivationPrefix,
			DerivationSuffix:  DerivationSuffix,
			SenderIdentityKey: UserIdentityKeyHex,
		}
	} else {
		outputSpec.InsertionRemittance = &wdk.BasketInsertion{
			Basket:             CustomBasket,
			CustomInstructions: to.Ptr("custom instructions"),
			Tags:               []primitives.StringUnder300{"tag1", "tag2"},
		}
	}

	return wdk.InternalizeActionArgs{
		Tx: atomicBeef,
		Outputs: []*wdk.InternalizeOutput{
			outputSpec,
		},
		Labels: []primitives.StringUnder300{
			"label1", "label2",
		},
		Description:    "description",
		SeekPermission: nil,
	}
}

func DefaultWalletInternalizeActionArgs(t *testing.T, protocol sdk.InternalizeProtocol) sdk.InternalizeActionArgs {
	t.Helper()

	spec := testabilities.GivenTX().WithInput(1000).WithP2PKHOutput(ExpectedValueToInternalize)

	atomicBeef, err := spec.TX().AtomicBEEF(false)
	require.NoError(t, err)

	outputSpec := sdk.InternalizeOutput{
		OutputIndex: 0,
		Protocol:    protocol,
	}
	if protocol == sdk.InternalizeProtocolWalletPayment {
		outputSpec.PaymentRemittance = &sdk.Payment{
			DerivationPrefix:  DerivationPrefixBytes,
			DerivationSuffix:  DerivationSuffixBytes,
			SenderIdentityKey: UserIdentityKey,
		}
	} else {
		outputSpec.InsertionRemittance = &sdk.BasketInsertion{
			Basket:             CustomBasket,
			CustomInstructions: "custom instructions",
			Tags:               []string{"tag1", "tag2"},
		}
	}

	return sdk.InternalizeActionArgs{
		Tx: atomicBeef,
		Outputs: []sdk.InternalizeOutput{
			outputSpec,
		},
		Labels: []string{
			"label1", "label2",
		},
		Description:    "description",
		SeekPermission: nil,
	}
}

// DefaultWalletInternalizeActionArgsMatchingBRC29 builds args where Tx's output locking script
// matches a BRC-29-derived address for the provided keyDeriver (wallet owner).
func DefaultWalletInternalizeActionArgsMatchingBRC29(t *testing.T, protocol sdk.InternalizeProtocol, keyDeriver *sdk.KeyDeriver) sdk.InternalizeActionArgs {
	t.Helper()

	args := DefaultWalletInternalizeActionArgs(t, protocol)

	if protocol == sdk.InternalizeProtocolWalletPayment {
		keyID := brc29.KeyID{DerivationPrefix: DerivationPrefix, DerivationSuffix: DerivationSuffix}
		lock, err := brc29.LockForSelf(brc29.PubHex(UserIdentityKeyHex), keyID, keyDeriver)
		require.NoError(t, err)

		spec := testabilities.GivenTX().WithInput(1000).WithOutputScript(ExpectedValueToInternalize, lock)
		args.Tx = spec.AtomicBEEF().Bytes()
	}

	return args
}

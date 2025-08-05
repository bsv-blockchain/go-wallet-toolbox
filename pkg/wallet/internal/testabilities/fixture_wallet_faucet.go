package testabilities

import (
	"fmt"
	"testing"

	primitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

type FaucetFixture interface {
	TopUp(satoshis satoshi.Value) (txtestabilities.TransactionSpec, *sdk.Payment)
}

type faucetFixture struct {
	testing.TB
	userWallet *wallet.Wallet
	index      int
}

func (f *faucetFixture) TopUp(satoshis satoshi.Value) (txtestabilities.TransactionSpec, *sdk.Payment) {
	f.Helper()

	senderPriv, senderPub := sdk.AnyoneKey()

	paymentRemittance := &sdk.Payment{
		DerivationPrefix:  fixtures.DerivationPrefixBytes,
		DerivationSuffix:  fixtures.DerivationSuffixBytes,
		SenderIdentityKey: senderPub,
	}

	keyID := brc29.KeyID{
		DerivationPrefix: fixtures.DerivationPrefix,
		DerivationSuffix: fixtures.DerivationSuffix,
	}

	recipientPubKey, err := f.userWallet.GetPublicKey(f.Context(), sdk.GetPublicKeyArgs{IdentityKey: true}, "")
	require.NoError(f, err, "Failed to derive public key for top up")

	lockingScript, err := brc29.Lock(senderPriv, keyID, recipientPubKey.PublicKey)
	require.NoError(f, err, "Failed to create locking script for top up")

	spec := txtestabilities.GivenTX().
		WithInput(satoshi.MustAdd(satoshis, 1).MustUInt64()).
		WithOutputScript(satoshis.MustUInt64(), lockingScript).
		WithOPReturn(fmt.Sprintf("faucet index %d", f.index))

	atomicBeef := spec.AtomicBEEF().Bytes()

	f.internalizeTopUp(atomicBeef, paymentRemittance)

	f.index++

	return spec, paymentRemittance
}

func (f *faucetFixture) internalizeTopUp(beef []byte, paymentRemittance *sdk.Payment) {
	action, err := f.userWallet.InternalizeAction(f.Context(), sdk.InternalizeActionArgs{
		Tx: beef,
		Outputs: []sdk.InternalizeOutput{{
			OutputIndex:       0,
			Protocol:          sdk.InternalizeProtocolWalletPayment,
			PaymentRemittance: paymentRemittance,
		},
		},
		Labels: []string{
			"faucet=mocked", "source=faucet",
		},
		Description: "funds from faucet",
	}, "")
	require.NoError(f, err, "failed to internalize top up transaction - check the test setup and InternalizeAction method")
	require.NotNil(f, action, "internalize action result should not be nil - check the InternalizeAction method")
	require.True(f, action.Accepted, "internalize action should accept the transaction - check the InternalizeAction method")
}

func brc29LockingScript(t testing.TB, userWallet sdk.Interface, senderKey *primitives.PublicKey, keyID string) *script.Script {
	targetPubKey, err := userWallet.GetPublicKey(t.Context(), sdk.GetPublicKeyArgs{
		ForSelf: to.Ptr(true),
		EncryptionArgs: sdk.EncryptionArgs{
			Counterparty: sdk.Counterparty{
				Type:         sdk.CounterpartyTypeOther,
				Counterparty: senderKey,
			},
			ProtocolID: brc29.Protocol,
			KeyID:      keyID,
		},
	}, "")
	require.NoError(t, err, "Failed to derive public key for top up - check the test setup and GetPublicKey method")

	address, err := script.NewAddressFromPublicKey(targetPubKey.PublicKey, false)
	require.NoError(t, err, "Failed to create address on which the top up should be sent")

	recipientPubKey, err := userWallet.GetPublicKey(t.Context(), sdk.GetPublicKeyArgs{IdentityKey: true}, "")
	require.NoError(t, err, "Failed to derive public key for top up")

	fmt.Println("ADDRESS: \n\n\n", address.AddressString, "\n\n\n===========================")
	fmt.Println("keyID: ", keyID)
	fmt.Println("sender: ", senderKey.ToDERHex())
	fmt.Println("recipient: ", recipientPubKey.PublicKey.ToDERHex())

	lockingScript, err := p2pkh.Lock(address)
	require.NoError(t, err, "Failed to create locking script for top up")
	return lockingScript
}

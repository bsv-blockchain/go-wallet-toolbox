package example_setup

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/utils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/go-softwarelab/common/pkg/slices"
)

// InternalizeFromFaucet is a helper function to internalize a transaction from the faucet
func InternalizeFromFaucet(ctx context.Context, atomicBeefBytes []byte, wallet sdk.Interface, alice *Setup) error {
	paymentRemittance := utils.DerivationParts()
	anyonePriv, anyonePub := sdk.AnyoneKey()

	keyID := brc29.KeyID{
		DerivationPrefix: base64.StdEncoding.EncodeToString(paymentRemittance.DerivationPrefix),
		DerivationSuffix: base64.StdEncoding.EncodeToString(paymentRemittance.DerivationSuffix),
	}

	// Generate the BRC29 address that is used to internalize the transaction output
	address, err := brc29.Address(
		anyonePriv,
		keyID,
		alice.IdentityKey,
		brc29.WithTestNet(),
	)
	if err != nil {
		panic(fmt.Errorf("failed to generate BRC29 address: %w", err))
	}

	addressObj, err := script.NewAddressFromString(address.AddressString)
	if err != nil {
		panic(fmt.Errorf("failed to parse address %q: %w", address.AddressString, err))
	}

	lockingScript, err := p2pkh.Lock(addressObj)
	if err != nil {
		panic(fmt.Errorf("failed to create locking script for address %q: %w", address, err))
	}

	tx, err := transaction.NewTransactionFromBEEF(atomicBeefBytes)
	if err != nil {
		panic(fmt.Errorf("failed to create transaction from atomic beef: %w", err))
	}

	// Checking all outputs in the transaction to find the ones that match the locking script related to the BRC29 address
	var vouts []int
	for vout, output := range tx.Outputs {
		if output.LockingScript.Equals(lockingScript) {
			vouts = append(vouts, vout)
		}
	}
	
	if len(vouts) == 0 {
		panic(fmt.Errorf("no outputs found for address %q in transaction %q", address.AddressString, tx.TxID().String()))
	}
	show.Info("Outputs matching to the derived address based on the payment remittance", vouts)


	internalizeArgs := sdk.InternalizeActionArgs{
		Tx: atomicBeefBytes,
		Outputs: slices.Map(vouts, func(vout int) sdk.InternalizeOutput {
			return sdk.InternalizeOutput{
				OutputIndex: uint32(vout),
				Protocol:    "wallet payment",
				PaymentRemittance: &sdk.Payment{
					DerivationPrefix:  paymentRemittance.DerivationPrefix,
					DerivationSuffix:  paymentRemittance.DerivationSuffix,
					SenderIdentityKey: anyonePub,
				},
			}
		}),
		Description: "internalize from faucet",
	}

	iar, err := wallet.InternalizeAction(ctx, internalizeArgs, "")
	if err != nil {
		show.WalletError("InternalizeAction", internalizeArgs, err)
		return err
	}

	show.WalletSuccess("InternalizeAction", internalizeArgs, *iar)
	return nil
}

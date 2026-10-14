package pos

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

type MerchantWallet interface {
	Signer
	ListActions(context.Context, sdk.ListActionsArgs, string) (*sdk.ListActionsResult, error)
	InternalizeAction(context.Context, sdk.InternalizeActionArgs, string) (*sdk.InternalizeActionResult, error)
}

// MerchantAcceptor uses the full signing wallet backed by cloud storage.
// A storage provider alone cannot perform merchant BRC-42 derivation.
// The configured toolbox storage/services must perform BEEF/SPV and broadcast
// validation; Accepted must be true before any signed receipt is issued.
func MerchantAcceptor(wallet MerchantWallet) func(context.Context, Frame, int64) (string, error) {
	return func(ctx context.Context, f Frame, amount int64) (string, error) {
		if amount <= 0 {
			return "", errors.New("payment amount must be positive")
		}
		raw, err := byteArray(f.Transaction, 2000000)
		if err != nil {
			return "", err
		}
		tx, err := transaction.NewTransactionFromBEEF(raw)
		if err != nil {
			return "", err
		}
		if int(f.OutputIndex) >= len(tx.Outputs) || tx.Outputs[f.OutputIndex].Satoshis != uint64(amount) {
			return "", errors.New("payment amount does not match")
		}
		sender, err := identity(f.SenderIdentityKey)
		if err != nil {
			return "", err
		}
		own := true
		derived, err := wallet.GetPublicKey(ctx, sdk.GetPublicKeyArgs{EncryptionArgs: sdk.EncryptionArgs{ProtocolID: sdk.Protocol{SecurityLevel: 2, Protocol: "3241645161d8"}, KeyID: f.DerivationPrefix + " " + f.DerivationSuffix, Counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: sender}}, ForSelf: &own}, originator)
		if err != nil {
			return "", err
		}
		if derived == nil || derived.PublicKey == nil {
			return "", errors.New("merchant derivation unavailable")
		}
		address, err := script.NewAddressFromPublicKey(derived.PublicKey, true)
		if err != nil {
			return "", err
		}
		locking, err := p2pkh.Lock(address)
		if err != nil {
			return "", err
		}
		if !bytes.Equal(*locking, *tx.Outputs[f.OutputIndex].LockingScript) {
			return "", errors.New("payment destination does not match")
		}
		prefix, err := base64.StdEncoding.DecodeString(f.DerivationPrefix)
		if err != nil {
			return "", err
		}
		suffix, err := base64.StdEncoding.DecodeString(f.DerivationSuffix)
		if err != nil {
			return "", err
		}
		result, err := wallet.InternalizeAction(ctx, sdk.InternalizeActionArgs{Tx: raw, Description: "Point of sale payment", Labels: []string{"pos " + tx.TxID().String()}, Outputs: []sdk.InternalizeOutput{{OutputIndex: f.OutputIndex, Protocol: sdk.InternalizeProtocolWalletPayment, PaymentRemittance: &sdk.Payment{DerivationPrefix: prefix, DerivationSuffix: suffix, SenderIdentityKey: sender}}}}, originator)
		if err != nil {
			return "", err
		}
		if result == nil || !result.Accepted {
			return "", errors.New("merchant wallet did not accept payment")
		}
		limit := uint32(10)
		actions, err := wallet.ListActions(ctx, sdk.ListActionsArgs{Labels: []string{"pos " + tx.TxID().String()}, Limit: &limit}, originator)
		if err != nil {
			return "", err
		}
		if actions != nil {
			for _, action := range actions.Actions {
				if action.Txid.String() == tx.TxID().String() && (action.Status == sdk.ActionStatusCompleted || action.Status == sdk.ActionStatusUnproven) {
					return tx.TxID().String(), nil
				}
			}
		}
		return "", errors.New("merchant payment broadcast is pending or failed")
	}
}

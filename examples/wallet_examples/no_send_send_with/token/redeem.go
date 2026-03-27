package token

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

// RedeemPushDropToken creates a transaction to redeem a PushDrop token using aliceWallet and associated token data.
func RedeemPushDropToken(ctx context.Context, aliceWallet wallet.Interface, token Token, noSendChange []transaction.Outpoint) (chainhash.Hash, []transaction.Outpoint) {
	t := pushdrop.PushDrop{
		Wallet: aliceWallet,
	}

	counterparty := wallet.Counterparty{
		Type:         wallet.CounterpartyTypeOther,
		Counterparty: token.FromIdentityKey,
	}

	unlocker := t.Unlock(ctx, protocolID, token.KeyID, counterparty, wallet.SignOutputsAll, false, pushdrop.UnlockOptions{SourceSatoshis: to.Ptr(token.Satoshis)})

	label := redeemPushDropTokenLabel

	createActionResult, err := aliceWallet.CreateAction(ctx, wallet.CreateActionArgs{
		InputBEEF: token.Beef,
		Inputs: []wallet.CreateActionInput{{
			Outpoint:              token.DataOutpoint(),
			UnlockingScriptLength: 73,
			InputDescription:      label,
		}},
		Options: &wallet.CreateActionOptions{
			NoSend:           to.Ptr(true),
			NoSendChange:     noSendChange,
			RandomizeOutputs: to.Ptr(false),
			SignAndProcess:   to.Ptr(false),
		},
		Labels:      []string{label},
		Description: label,
	}, "")
	if err != nil {
		panic(err)
	}

	if createActionResult.SignableTransaction == nil {
		panic("createAction returned nil SignableTransaction")
	}

	signableTx, err := transaction.NewTransactionFromBEEF(createActionResult.SignableTransaction.Tx)
	if err != nil {
		panic(err)
	}

	unlockingScript, err := unlocker.Sign(signableTx, 0)
	if err != nil {
		panic(fmt.Errorf("unable to sign tx: %w", err))
	}

	signActionResult, err := aliceWallet.SignAction(ctx, wallet.SignActionArgs{
		Reference: createActionResult.SignableTransaction.Reference,
		Spends:    map[uint32]wallet.SignActionSpend{0: {UnlockingScript: unlockingScript.Bytes()}},
		Options: &wallet.SignActionOptions{
			AcceptDelayedBroadcast: to.Ptr(false),
		},
	}, "")
	if err != nil {
		panic(err)
	}

	signedTxID := signActionResult.Txid
	// Because txid changes after signing, we need to adjust noSendChange outpoints to use the new txid
	nextNoSendChange := replaceTxIDInOutpoints(to.Value(signableTx.TxID()), signedTxID, createActionResult.NoSendChange)

	show.Info("Redeemed Token", signedTxID.String())

	return signedTxID, nextNoSendChange
}

func replaceTxIDInOutpoints(old, new chainhash.Hash, outpoints []transaction.Outpoint) []transaction.Outpoint {
	adjusted := make([]transaction.Outpoint, len(outpoints))
	for i, op := range outpoints {
		if op.Txid.Equal(old) {
			adjusted[i] = transaction.Outpoint{
				Txid:  new,
				Index: op.Index,
			}
		} else {
			adjusted[i] = op
		}
	}
	return adjusted
}

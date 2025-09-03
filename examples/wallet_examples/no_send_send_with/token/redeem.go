package token

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
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

	beefBytes, err := token.Beef.Bytes()
	if err != nil {
		panic(err)
	}

	createActionResult, err := aliceWallet.CreateAction(ctx, wallet.CreateActionArgs{
		InputBEEF: beefBytes,
		Inputs: []wallet.CreateActionInput{{
			Outpoint:              token.DataOutpoint(),
			UnlockingScriptLength: 73,
			InputDescription:      label,
		}},
		Options: &wallet.CreateActionOptions{
			NoSend:           to.Ptr(true),
			NoSendChange:     noSendChange,
			RandomizeOutputs: to.Ptr(false),
		},
		Labels:      []string{label},
		Description: label,
	}, "")
	if err != nil {
		panic(err)
	}

	signableTx := createActionResult.SignableTransaction
	if signableTx == nil {
		panic("createAction returned nil SignableTransaction")
	}

	beef, txID, err := transaction.NewBeefFromAtomicBytes(signableTx.Tx)
	if err != nil {
		panic(err)
	}

	tx := beef.FindAtomicTransactionByHash(txID)
	if tx == nil {
		panic(fmt.Sprintf("failed to find transaction with hash %s in BEEF data", txID.String()))
	}

	unlockingScript, err := unlocker.Sign(tx, 0)
	if err != nil {
		panic(fmt.Errorf("unable to sign tx: %w", err))
	}

	_, err = aliceWallet.SignAction(ctx, wallet.SignActionArgs{
		Reference: signableTx.Reference,
		Spends:    map[uint32]wallet.SignActionSpend{0: {UnlockingScript: unlockingScript.Bytes()}},
		Options: &wallet.SignActionOptions{
			AcceptDelayedBroadcast: to.Ptr(false),
		},
	}, "")
	if err != nil {
		panic(err)
	}

	return *txID, createActionResult.NoSendChange
}

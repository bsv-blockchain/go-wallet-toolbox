package main

import (
	"context"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/go-softwarelab/common/pkg/to"
)

func mintPushDropToken(ctx context.Context, aliceIdentityKey *ec.PublicKey, aliceWallet wallet.Interface, dataPrefix []byte, keyID string, counter int, noSendChange []transaction.Outpoint) Token {
	t := pushdrop.PushDrop{
		Wallet:     aliceWallet,
		Originator: "",
	}

	dataField := append(dataPrefix, []byte(fmt.Sprintf("%d", counter))...)
	fields := [][]byte{dataField}

	counterparty := wallet.Counterparty{
		Type: wallet.CounterpartyTypeSelf,
	}

	lockingScript, err := t.Lock(ctx, fields, protocolID, keyID, counterparty, true, false, pushdrop.LockBefore)
	if err != nil {
		panic(err)
	}

	show.Info("Mint token, Locking Script", lockingScript.String())

	label := mintPushDropTokenLabel
	satoshis := uint64(mintPushDropTokenSatoshis)

	createActionResult, err := aliceWallet.CreateAction(ctx, wallet.CreateActionArgs{
		Outputs: []wallet.CreateActionOutput{
			{
				LockingScript:      lockingScript.Bytes(),
				Satoshis:           satoshis,
				OutputDescription:  label,
				Tags:               []string{"relinquish"},
				CustomInstructions: pushDropCustomInstructions(keyID).JSON(),
			},
		},
		Options: &wallet.CreateActionOptions{
			NoSend:                 to.Ptr(true),
			NoSendChange:           noSendChange,
			RandomizeOutputs:       to.Ptr(false),
			AcceptDelayedBroadcast: to.Ptr(false),
		},
		Labels:      []string{label},
		Description: label,
	}, "")
	if err != nil {
		panic(err)
	}

	return Token{
		CreateActionResult: *createActionResult,
		KeyID:              keyID,
		FromIdentityKey:    aliceIdentityKey,
		Satoshis:           satoshis,
	}
}

package token

import (
	"context"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

// MintPushDropToken mints a PushDrop token using the provided context, identity key, wallet, and other parameters.
// It generates the token's locking script with associated data by invoking the pushdrop protocol.
// The function constructs an action in the wallet with specific outputs, disabling immediate broadcasting. It returns a Token.
func MintPushDropToken(
	ctx context.Context,
	aliceIdentityKey *ec.PublicKey,
	aliceWallet wallet.Interface,
	dataField []byte,
	keyID string,
	noSendChange []transaction.Outpoint,
) (Token, []transaction.Outpoint) {
	t := pushdrop.PushDrop{
		Wallet:     aliceWallet,
		Originator: "",
	}

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
				Tags:               []string{"mint"},
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
		TxID:            createActionResult.Txid,
		Beef:            createActionResult.Tx,
		KeyID:           keyID,
		FromIdentityKey: aliceIdentityKey,
		Satoshis:        satoshis,
	}, createActionResult.NoSendChange
}

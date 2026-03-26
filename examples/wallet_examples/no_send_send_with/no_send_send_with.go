package main

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/wallet_examples/no_send_send_with/token"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
)

const (
	tokensCount = 3
	dataPrefix  = "exampletoken"
)

var rand = randomizer.NewTestRandomizer()

// This example shows how to construct multiple transactions without broadcasting them immediately (NoSend),
// chain their internal change across steps (NoSendChange), and then broadcast them together in a single batch using SendWith.
// The demo uses simple PushDrop "tokens" to make the flow concrete.
func main() {
	show.ProcessStart("NoSend and SendWith Example based on PushDrop Tokens")
	ctx := context.Background()

	// Create Alice's wallet instance
	alice := example_setup.CreateAlice()

	// Create the wallet interface and establish database connection
	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	keyID := randomKeyID()

	tokens := mint(ctx, alice, aliceWallet, keyID)

	redeem(ctx, tokens, aliceWallet)
}

func mint(ctx context.Context, alice *example_setup.Setup, aliceWallet wallet.Interface, keyID string) token.Tokens {
	var prevNoSentChange []transaction.Outpoint
	tokens := make(token.Tokens, 0, tokensCount)

	show.Step("Mint multiple tokens", "all mints are done with noSend = true, so they are not broadcasted immediately")
	// Mint multiple tokens with noSend = true, each time passing the change from the previous mint as noSendChange to the next mint
	// This way we ensure that all mints will be broadcasted in a single batch
	for counter := range tokensCount {
		dataField := []byte(fmt.Sprintf("%s-%d", dataPrefix, counter))

		tok, noSendChangeOutpoints := token.MintPushDropToken(
			ctx,
			alice.IdentityKey,
			aliceWallet,
			dataField,
			keyID,
			prevNoSentChange,
		)

		tokens = append(tokens, tok)
		prevNoSentChange = noSendChangeOutpoints

		show.Info("Minted Token", tok.TxID.String())
	}

	show.Step("Broadcast all mints in a single batch using sendWith", "all mints are now broadcasted in a single batch using sendWith")
	// Now send all the mints in a single batch using sendWith
	sendWith(ctx, aliceWallet, tokens.TxIDs())

	show.Success("All tokens minted and broadcasted")

	return tokens
}

func redeem(ctx context.Context, tokens token.Tokens, aliceWallet wallet.Interface) {
	show.Step("Redeem multiple tokens", "all redeems are done with noSend = true, so they are not broadcasted immediately")
	// Redeem multiple tokens with noSend = true, each time passing the change from the previous redeem as noSendChange to the next redeem
	// This way we ensure that all redeems will be broadcasted in a single batch
	// We also collect the txIDs of all redeems to use them in sendWith later
	var prevNoSentChange []transaction.Outpoint
	redeemed := make([]chainhash.Hash, 0, len(tokens))
	for _, tok := range tokens {
		redeemedTxID, noSendChange := token.RedeemPushDropToken(
			ctx,
			aliceWallet,
			tok,
			prevNoSentChange,
		)

		redeemed = append(redeemed, redeemedTxID)
		prevNoSentChange = noSendChange
	}

	show.Step("Broadcast all redeems in a single batch using sendWith", "all redeems are now broadcasted in a single batch using sendWith")
	// Now send all the redeems in a single batch using sendWith
	sendWith(ctx, aliceWallet, redeemed)

	show.Success("All tokens redeemed and broadcasted")
}

func sendWith(ctx context.Context, aliceWallet wallet.Interface, txIDs []chainhash.Hash) {
	_, err := aliceWallet.CreateAction(ctx, wallet.CreateActionArgs{
		Options: &wallet.CreateActionOptions{
			SendWith: txIDs,
		},
		Description: "sendWith",
	}, "")
	if err != nil {
		panic(err)
	}
}

func randomKeyID() string {
	const keyIDLength = 8
	keyID, err := rand.Base64(keyIDLength)
	if err != nil {
		panic(err)
	}

	return keyID
}

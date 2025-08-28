package main

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
)

const (
	tokensCount = 3
)

var rand = randomizer.NewTestRandomizer()

func main() {
	show.ProcessStart("List Outputs")
	ctx := context.Background()

	// Create Alice's wallet instance
	alice := example_setup.CreateAlice()

	// Create the wallet interface and establish database connection
	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	dataPrefix := randomDataPrefix()
	keyID := randomKeyID()

	var prevNoSentChange []transaction.Outpoint
	var tokens Tokens

	// Mint multiple tokens with noSend = true, each time passing the change from the previous mint as noSendChange to the next mint
	// This way we ensure that all mints will be broadcasted in a single batch
	for counter := range tokensCount {
		token := mintPushDropToken(
			ctx,
			alice.IdentityKey,
			aliceWallet,
			dataPrefix,
			keyID,
			counter,
			prevNoSentChange,
		)

		tokens = append(tokens, token)
		prevNoSentChange = token.NoSendChange
	}

	// Now send all the mints in a single batch using sendWith
	sendWith(ctx, aliceWallet, tokens.TxIDs())

	// redeem the tokens
	prevNoSentChange = nil
	redeemed := make([]chainhash.Hash, 0, len(tokens))
	for _, token := range tokens {
		noSendChange, redeemedTxID := redeemPushDropToken(
			ctx,
			aliceWallet,
			token,
			prevNoSentChange,
		)

		redeemed = append(redeemed, redeemedTxID)
		prevNoSentChange = noSendChange
	}

	// Now send all the redeems in a single batch using sendWith
	sendWith(ctx, aliceWallet, redeemed)
}

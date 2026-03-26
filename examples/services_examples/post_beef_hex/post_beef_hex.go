package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// This example demonstrates broadcasting a BSV transaction from existing BEEF hex data
// Focuses purely on the broadcasting mechanism with pre-encoded BEEF data
func main() {
	const (
		// this transaction is already broadcasted, set your own txID and beefHex
		transactionID = "c7218bcddee6e7a2ad097007d50831837bb174ad78c078f65260d7971a46d620"
		beefHex       = "0200beef01fef695190002020200eb0cb460644bf9b4d9b81d507fa6f462d2f3233c5c6c3b0b0884fc95192eecfe030206f4f2d417c9b4cf1d6a6f540ae41cc68aa409ffbe58bace7771a539d2b8b23801000040ddb69ed38c0b056b74827e4f84f4b018e112df8785d618b3ca96379596383602010001000000012f63dbc37992a6b72cc804d9376e4298d3a88e11a936d0b8619b33045629102c060000006a4730440220416565614ec6143399dcfe6a26af2aa82b8bb85d633419e767fa1402c080e3df02207db40dc0ded885b8419e04a180893a00e8ca613c7c09bd7fccdc1b0df6637e834121031ee1b94025e871d06c183319a4cab9b2007f3905b8245dc30b43883bcd9a7cfcffffffff022c010000000000001976a9144b0d6cbef5a813d2d12dcec1de2584b250dc96a388ac4a040000000000001976a91409c7c8eb4f1c4f4a4b375419c75851fe5af9965588ac0000000000010000000106f4f2d417c9b4cf1d6a6f540ae41cc68aa409ffbe58bace7771a539d2b8b238000000006a473044022038b20584ca33760448944e9858eb79c5db8244e2618035488579418d35a8ec2102205b12dec8d3d4fd6c77db7d9d0503dfa9094a5bef45cf91c8d4bd76c62192c82c412103440705ae05df8749a902c568f378167722afd396529aa03b96c4a55b7e7d857effffffff012b010000000000001976a914dd5aa593431e1ee683ce8e7d90135456444e681688ac00000000"
		network       = defs.NetworkTestnet
	)

	show.ProcessStart("Post BEEF Hex")

	// //Set to LevelDebug to see http request logs
	// slog.SetLogLoggerLevel(slog.LevelDebug)

	show.Step("Transaction", "parsing BEEF hex data")

	beefBytes, err := hex.DecodeString(beefHex)
	if err != nil {
		panic(fmt.Errorf("could not decode beef hex: %w", err))
	}

	beef, err := transaction.NewBeefFromBytes(beefBytes)
	if err != nil {
		panic(fmt.Errorf("could not create beef from bytes: %w", err))
	}

	serviceCfg := defs.DefaultServicesConfig(network)
	walletServices := services.New(slog.Default(), serviceCfg)

	show.Step("Wallet-Services", fmt.Sprintf("broadcasting transaction %s", transactionID))

	results, err := walletServices.PostFromBEEF(context.Background(), beef, []string{transactionID})
	if err != nil {
		panic(err)
	}

	show.Success("Posted BEEF to services")
	show.PostBEEFOutput(results)

	show.ProcessComplete("Post BEEF Hex")
}

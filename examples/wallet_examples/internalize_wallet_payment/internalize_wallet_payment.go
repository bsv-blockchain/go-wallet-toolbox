package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

var (
	// AtomicBeefHex is the transaction data in atomic beef hex format
	AtomicBeefHex = "" // example: 01010101c8c06c5fac63510b2b02ccab974a6ef0b0a4910dd8e881c06964f2b52d7ff4150200beef01fe849e19000c02fdce0a00ac05565e579d8c4257313d90ce7bea754aa41add817a0616a9199c84a89d2733fdcf0a02c8c06c5fac63510b2b02ccab974a6ef0b0a4910dd8e881c06964f2b52d7ff41501fd66050072958bee9c51d1a7511759ef6c73aa03a0533749e887c06514504c466a185fc201fdb202004d106b759b760b423b05be8e53b7ccd44db1cf8c39fd609ee70c316b0a2964df01fd5801003521b209685ae64f5a7f41fcfc5d487fe1b0162ee5a311620c252f6f48714ad101ad000af15dea439d12d3330dc65b5fac8a8e786d80c9f4e8c10dec91807c2fa085380157000e5a9d088abcc6bfb57f6aeb7f12a4fbe63fa07477bef78fa87f967466c374d4012a0083c8207772fb8586071053e855af973a2cce232d45d6a90e3ad403015a003ad1011400074cd69e726d1f7b9f7f1f301f701eef3dd36cbec654ddf7ee897d2567fbc2d4010b00620e7f9bd848d9123aad73e7b28b05e830eaf7d7188f3322d79b2256934026f9010400bbb0cac6a484ac94f774b0c795fa9f116f8251e7cdfe7b374938b8806563f383010300a7074e5aa1e7ffc5754fb762c7853adc20a80c25fad6ad60ddbf80b6d8ad02e40100009c956d581a811c8d45a81b716fbb9c4349c4be011f24c44d236a493b24ca5f42010100010000000122ffae11e662c209b8cc5ecce312af425b06f44668d667bb8b09fc04f1e25653010000006b483045022100bac3cea0816c2c8863b6a5207bef9c2236716c58140448d980241f851705872f02201a912284e254e76f33fab9b82eb577921950e9091edf4e79d9a6d00453a23a4e41210231c72ef229534d40d08af5b9a586b619d0b2ee2ace2874339c9cbcc4a79281c0ffffffff0201000000000000001976a914d430654b50459aa04e308c07daf4871185efdc3088ac0d000000000000001976a914cd5ea7065a42329a574b1eb7af9fbbca8a94e44b88ac00000000

	// Originator specifies the originator domain or FQDN used to identify the source of the action listing request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	Originator = "example.com"

	// Prefix is the derivation prefix for the payment remittance
	Prefix = "" // example: SfKxPIJNgdI=

	// Suffix is the derivation suffix for the payment remittance
	Suffix = "" // example: NaGLC6fMH50=

	// IdentityKey is the sender identity key for the payment remittance
	IdentityKey = "" // example: 0231c72ef229534d40d08af5b9a586b619d0b2ee2ace2874339c9cbcc4a79281c0
)

// This example demonstrates how to internalize a transaction into Alice's wallet.
// AtomicBeefHex, IdentityKey, Prefix, and Suffix are required to internalize a transaction.
func main() {
	show.ProcessStart("Internalize Wallet Payment")
	ctx := context.Background()

	if Prefix == "" || Suffix == "" || AtomicBeefHex == "" || IdentityKey == "" {
		panic("Prefix, Suffix, AtomicBeefHex, and IdentityKey are required")
	}

	show.Step("Alice", "Creating wallet and setting up environment")

	alice := example_setup.CreateAlice()

	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	derivationPrefix, err := base64.StdEncoding.DecodeString(Prefix)
	if err != nil {
		panic(fmt.Errorf("failed to decode derivation prefix: %w", err))
	}

	derivationSuffix, err := base64.StdEncoding.DecodeString(Suffix)
	if err != nil {
		panic(fmt.Errorf("failed to decode derivation suffix: %w", err))
	}

	senderIdentityKey, err := ec.PublicKeyFromString(IdentityKey)
	if err != nil {
		panic(fmt.Errorf("failed to get sender identity key: %w", err))
	}

	decodedBeef, err := hex.DecodeString(AtomicBeefHex)
	if err != nil {
		panic(fmt.Errorf("failed to decode beef: %w", err))
	}

	// Create internalization arguments with payment remittance configuration
	internalizeArgs := sdk.InternalizeActionArgs{
		Tx: decodedBeef,
		Outputs: []sdk.InternalizeOutput{
			{
				OutputIndex: 0,
				Protocol:    "wallet payment",
				PaymentRemittance: &sdk.Payment{
					DerivationPrefix:  derivationPrefix,
					DerivationSuffix:  derivationSuffix,
					SenderIdentityKey: senderIdentityKey,
				},
			},
		},
		Description: "internalize transaction",
	}

	show.Step("Alice", "Internalizing transaction")

	// Execute the internalization to add external transaction to wallet history
	result, err := aliceWallet.InternalizeAction(ctx, internalizeArgs, Originator)
	if err != nil {
		panic(fmt.Errorf("failed to internalize action: %w", err))
	}

	show.WalletSuccess("InternalizeAction", internalizeArgs, *result)
	show.Success("Transaction internalized successfully")
	show.ProcessComplete("Internalize Wallet Payment")
}

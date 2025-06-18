package example_setup

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/utils"
	"github.com/bsv-blockchain/go-sdk/script"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func FaucetAddress(wallet *Setup) string {
	parts := utils.DerivationParts()

	senderKeyDeriver := sdk.NewKeyDeriver(&wallet.privateKey)

	recipientIdentityKey := wallet.identityKey

	walletPaymentProtocol := sdk.Protocol{
		SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty,
		Protocol:      "wallet payment",
	}

	// Derive the public key using the same pattern as BRC29
	key, err := senderKeyDeriver.DerivePublicKey(walletPaymentProtocol, parts.KeyID, sdk.Counterparty{
		Type:         sdk.CounterpartyTypeSelf,
		Counterparty: &recipientIdentityKey,
	}, false)
	if err != nil {
		panic(fmt.Errorf("failed to derive public key: %w", err))
	}

	// Generate testnet address from the derived public key
	isMainnet := false // Force testnet
	address, err := script.NewAddressFromPublicKey(key, isMainnet)
	if err != nil {
		panic(fmt.Errorf("failed to generate address: %w", err))
	}

	fmt.Println("====================================")
	fmt.Println("")
	fmt.Println("Below is the address that you should top up from faucet:")
	fmt.Println("")
	fmt.Println(address.AddressString)
	fmt.Println("")
	fmt.Println("You can use one of those testnet faucets:")
	fmt.Println("https://scrypt.io/faucet")
	fmt.Println("https://witnessonchain.com/faucet/tbsv")

	return address.AddressString
}

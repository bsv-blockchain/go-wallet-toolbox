package example_setup

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/utils"
	"github.com/bsv-blockchain/go-sdk/script"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func FaucetAddress(wallet *Setup) string {
	parts := utils.DerivationParts()

	senderKeyDeriver := sdk.NewKeyDeriver(&wallet.PrivateKey)

	recipientIdentityKey := wallet.IdentityKey

	walletPaymentProtocol := sdk.Protocol{
		SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty,
		Protocol:      "wallet payment",
	}

	key, err := senderKeyDeriver.DerivePublicKey(walletPaymentProtocol, parts.KeyID, sdk.Counterparty{
		Type:         sdk.CounterpartyTypeSelf,
		Counterparty: &recipientIdentityKey,
	}, false)
	if err != nil {
		panic(fmt.Errorf("failed to derive public key: %w", err))
	}

	
	isMainnet := false
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

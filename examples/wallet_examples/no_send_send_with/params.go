package main

import "github.com/bsv-blockchain/go-sdk/wallet"

const (
	protocolName = "nosendexample"
	mintPushDropTokenLabel = "mintPushDropToken"
	mintPushDropTokenSatoshis = 37

	redeemPushDropTokenLabel = "redeemPushDropToken"
)

var protocolID = wallet.Protocol{
	SecurityLevel: wallet.SecurityLevelEveryAppAndCounterparty,
	Protocol:      protocolName,
}

func randomKeyID() string {
	const keyIDLength = 8
	keyID, err := rand.Base64(keyIDLength)
	if err != nil {
		panic(err)
	}

	return keyID
}

func randomDataPrefix() []byte {
	const dataPrefixLength = 11
	dataPrefix, err := rand.Bytes(dataPrefixLength)
	if err != nil {
		panic(err)
	}

	return dataPrefix
}

func pushDropCustomInstructions(keyID string) *customInstructions {
	return &customInstructions{
		ProtocolID: customInstructionsProtocolID{
			SecurityLevel: int(protocolID.SecurityLevel),
			Protocol:      protocolID.Protocol,
		},
		KeyID:        keyID,
		Counterparty: "self",
		Type:         "PushDrop",
	}
}

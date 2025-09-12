package token

import (
	"encoding/json"

	"github.com/bsv-blockchain/go-sdk/wallet"
)

const (
	protocolName              = "nosendexample"
	mintPushDropTokenLabel    = "mintPushDropToken"
	mintPushDropTokenSatoshis = 37

	redeemPushDropTokenLabel = "redeemPushDropToken"
)

var protocolID = wallet.Protocol{
	SecurityLevel: wallet.SecurityLevelEveryAppAndCounterparty,
	Protocol:      protocolName,
}

type customInstructionsProtocolID struct {
	SecurityLevel int    `json:"securityLevel,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

type customInstructions struct {
	ProtocolID   customInstructionsProtocolID `json:"protocolID,omitempty"`
	KeyID        string                       `json:"keyID,omitempty"`
	Counterparty string                       `json:"counterparty,omitempty"`
	Type         string                       `json:"type,omitempty"`
}

func (c *customInstructions) JSON() string {
	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}

	return string(data)
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

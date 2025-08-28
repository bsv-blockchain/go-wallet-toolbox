package main

import "encoding/json"

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

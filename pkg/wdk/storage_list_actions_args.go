package wdk

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
)

// ListActionsArgs defines arguments for listing actions (transactions)
type ListActionsArgs struct {
	Labels                           []primitives.StringUnder300                 `json:"labels,omitempty"`
	Limit                            primitives.PositiveIntegerDefault10Max10000 `json:"limit,omitempty"`
	Offset                           primitives.PositiveIntegerDefault10Max10000 `json:"offset,omitempty"`
	LabelQueryMode                   *primitives.LabelQueryModeString            `json:"labelQueryMode"`            // "all=true" or "any=false"
	SeekPermissions                  *primitives.BooleanDefaultTrue              `json:"seekPermissions,omitempty"` // If false, operation is not allowed
	IncludeInputs                    *primitives.BooleanDefaultFalse             `json:"includeInputs,omitempty"`
	IncludeOutputs                   *primitives.BooleanDefaultFalse             `json:"includeOutputs,omitempty"`
	IncludeLabels                    *primitives.BooleanDefaultFalse             `json:"includeLabels,omitempty"`
	IncludeInputSourceLockingScripts *primitives.BooleanDefaultFalse             `json:"includeInputSourceLockingScripts,omitempty"`
	IncludeInputUnlockingScripts     *primitives.BooleanDefaultFalse             `json:"includeInputUnlockingScripts,omitempty"`
	IncludeOutputLockingScripts      *primitives.BooleanDefaultFalse             `json:"includeOutputLockingScripts,omitempty"`
}

// ListActionsResult defines the result of listing actions
type ListActionsResult struct {
	TotalActions uint32         `json:"totalActions"`
	Actions      []WalletAction `json:"actions"`
}

// WalletAction represents a transaction in the wallet
type WalletAction struct {
	TxID        string               `json:"txid"`
	Satoshis    int64                `json:"satoshis"`
	Status      string               `json:"status"`
	IsOutgoing  bool                 `json:"isOutgoing"`
	Description string               `json:"description"`
	Version     uint32               `json:"version"`
	LockTime    uint32               `json:"lockTime"`
	Labels      []string             `json:"labels"`
	Inputs      []WalletActionInput  `json:"inputs"`
	Outputs     []WalletActionOutput `json:"outputs"`
}

// WalletActionInput represents an input in a wallet action
type WalletActionInput struct {
	SourceOutpoint      string `json:"sourceOutpoint"`
	SourceSatoshis      uint64 `json:"sourceSatoshis"`
	InputDescription    string `json:"inputDescription"`
	SequenceNumber      uint32 `json:"sequenceNumber"`
	SourceLockingScript string `json:"sourceLockingScript,omitempty"`
	UnlockingScript     string `json:"unlockingScript,omitempty"`
}

// WalletActionOutput represents an output in a wallet action
type WalletActionOutput struct {
	Satoshis           uint64   `json:"satoshis"`
	Spendable          bool     `json:"spendable"`
	OutputIndex        uint32   `json:"outputIndex"`
	OutputDescription  string   `json:"outputDescription"`
	Basket             string   `json:"basket"`
	Tags               []string `json:"tags,omitempty"`
	LockingScript      string   `json:"lockingScript,omitempty"`
	CustomInstructions string   `json:"customInstructions,omitempty"`
}

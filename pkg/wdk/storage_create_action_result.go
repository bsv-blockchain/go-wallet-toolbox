package wdk

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"

// StorageCreateTransactionSdkInput represents the input for SDK transaction creation
type StorageCreateTransactionSdkInput struct {
	Vin                   int                          `json:"vin"`
	SourceTxID            string                       `json:"sourceTxid"`
	SourceVout            uint32                       `json:"sourceVout"`
	SourceSatoshis        int64                        `json:"sourceSatoshis"`
	SourceLockingScript   string                       `json:"sourceLockingScript"`
	SourceTransaction     primitives.ExplicitByteArray `json:"sourceTransaction,omitempty"`
	UnlockingScriptLength *primitives.PositiveInteger  `json:"unlockingScriptLength"`
	ProvidedBy            ProvidedBy                   `json:"providedBy"`
	Type                  OutputType                   `json:"type"`
	SpendingDescription   *string                      `json:"spendingDescription,omitempty"`
	DerivationPrefix      *string                      `json:"derivationPrefix,omitempty"`
	DerivationSuffix      *string                      `json:"derivationSuffix,omitempty"`
	SenderIdentityKey     *string                      `json:"senderIdentityKey,omitempty"`
}

// StorageCreateTransactionSdkOutput represents the output for SDK transaction creation
type StorageCreateTransactionSdkOutput struct {
	ValidCreateActionOutput
	// Additional fields
	Vout             uint32     `json:"vout"`
	ProvidedBy       ProvidedBy `json:"providedBy"`
	Purpose          string     `json:"purpose"`
	DerivationSuffix *string    `json:"derivationSuffix"`
}

// StorageCreateActionResult represents the result of creating a transaction action
type StorageCreateActionResult struct {
	InputBeef               primitives.ExplicitByteArray         `json:"inputBeef"`
	Inputs                  []*StorageCreateTransactionSdkInput  `json:"inputs"`
	Outputs                 []*StorageCreateTransactionSdkOutput `json:"outputs"`
	NoSendChangeOutputVouts []int                                `json:"noSendChangeOutputVouts"`
	DerivationPrefix        string                               `json:"derivationPrefix"`
	Version                 uint32                               `json:"version"`
	LockTime                uint32                               `json:"lockTime"`
	Reference               string                               `json:"reference"`
}

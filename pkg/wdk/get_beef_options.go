package wdk

import (
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// StorageGetBeefOptions defines options to customize retrieval and filtering behavior for beef data from storage sources.
type StorageGetBeefOptions struct {
	TrustSelf       wallet.TrustSelf             `json:"trustSelf,omitempty"`
	KnownTxIDs      []string                     `json:"knownTxids,omitempty"`
	MergeToBeef     primitives.ExplicitByteArray `json:"mergeToBeef,omitempty"`
	IgnoreStorage   bool                         `json:"ignoreStorage,omitempty"`
	IgnoreServices  bool                         `json:"ignoreServices,omitempty"`
	IgnoreNewProven bool                         `json:"ignoreNewProven,omitempty"`
	MinProofLevel   int                          `json:"minProofLevel,omitempty"`
}

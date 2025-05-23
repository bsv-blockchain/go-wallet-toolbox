package wdk

import "github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"

// ListOutputsArgs defines the query parameters for listing outputs
type ListOutputsArgs struct {
	Basket                    primitives.StringUnder300                   `json:"basket,omitempty"`
	Tags                      []primitives.StringUnder300                 `json:"tags,omitempty"`
	TagQueryMode              string                                      `json:"tagQueryMode,omitempty"` // 'all' or 'any'
	IncludeLockingScripts     bool                                        `json:"includeLockingScripts,omitempty"`
	IncludeTransactions       bool                                        `json:"includeTransactions,omitempty"`
	IncludeCustomInstructions bool                                        `json:"includeCustomInstructions,omitempty"`
	IncludeTags               bool                                        `json:"includeTags,omitempty"`
	IncludeLabels             bool                                        `json:"includeLabels,omitempty"`
	Limit                     primitives.PositiveIntegerDefault10Max10000 `json:"limit"`
	Offset                    primitives.PositiveInteger                  `json:"offset"`
	SeekPermission            bool                                        `json:"seekPermission,omitempty"`
	KnownTxids                []string                                    `json:"knownTxids,omitempty"`
}

// WalletOutput represents an output returned from listOutputs
type WalletOutput struct {
	Satoshis           primitives.SatoshiValue     `json:"satoshis"`
	Spendable          bool                        `json:"spendable"`
	Outpoint           primitives.OutpointString   `json:"outpoint"`
	CustomInstructions *string                     `json:"customInstructions,omitempty"`
	LockingScript      *primitives.HexString       `json:"lockingScript,omitempty"`
	Tags               []primitives.StringUnder300 `json:"tags,omitempty"`
	Labels             []primitives.StringUnder300 `json:"labels,omitempty"`
}

// ListOutputsResult contains the result of listing wallet outputs
type ListOutputsResult struct {
	TotalOutputs primitives.PositiveInteger `json:"totalOutputs"`
	BEEF         *primitives.BEEF           `json:"BEEF,omitempty"`
	Outputs      []*WalletOutput            `json:"outputs"`
}

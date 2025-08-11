package entity

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"

// ListOutputsFilter is the filter used to fetch outputs from repo
type ListOutputsFilter struct {
	Basket                    string
	Limit                     int
	Offset                    int
	UserID                    int
	IncludeTags               bool
	IncludeLockingScripts     bool
	IncludeCustomInstructions bool
	Tags                      []string
	TagsQueryMode             defs.QueryMode
	IncludeSpent              bool
}

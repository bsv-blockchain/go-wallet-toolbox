package entity

// ListOutputsFilter is the filter used to fetch outputs from repo
type ListOutputsFilter struct {
	Basket                    string
	Tags                      []string
	TagQueryMode              string
	IncludeLockingScripts     bool
	IncludeTransactions       bool
	IncludeCustomInstructions bool
	IncludeTags               bool
	IncludeLabels             bool
	Limit                     int
	Offset                    int
	KnownTxids                []string
}

type ListOutputsParams struct {
	UserID        int
	BasketName    string
	KnownTxids    []string
	Limit, Offset int
}

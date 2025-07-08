package wdk

import "fmt"

// ScriptUsageStatusResult represents the result of a script usage status query
type ScriptUsageStatusResult struct {
	Name       string `json:"name"`
	IsUsed     bool   `json:"isUsed"`
	ScriptHash string `json:"scriptHash"`
}

// ScriptHistoryItem represents a single transaction in script history
type ScriptHistoryItem struct {
	TxHash string `json:"tx_hash"`
	Height *int   `json:"height,omitempty"`
}

// ScriptHistoryResult represents the result of a script history query
type ScriptHistoryResult struct {
	Name       string              `json:"name"`       // Service name that provided the result
	ScriptHash string              `json:"scriptHash"` // The script hash that was queried
	History    []ScriptHistoryItem `json:"history"`    // Array of transactions using this script
}

// BulkScriptHistoryResult represents the result of a bulk script history query
type BulkScriptHistoryResult struct {
	Name    string                         `json:"name"`    // Service name that provided the result
	Results map[string]ScriptHistoryResult `json:"results"` // Map of script hash to its history result
}

// CombinedScriptHistoryResult represents both confirmed and unconfirmed script history
type CombinedScriptHistoryResult struct {
	Name               string              `json:"name"`               // Service name that provided the result
	ScriptHash         string              `json:"scriptHash"`         // The script hash that was queried
	ConfirmedHistory   ScriptHistoryResult `json:"confirmedHistory"`   // Confirmed transactions
	UnconfirmedHistory ScriptHistoryResult `json:"unconfirmedHistory"` // Unconfirmed transactions
}

// ScriptHashHistoryResponse represents the response from the script history endpoints
type ScriptHashHistoryResponse struct {
	// ScriptHash is the script hash for which the history is being retrieved (not always present)
	ScriptHash string `json:"script,omitempty"`

	// Result contains the history of transactions associated with the script hash.
	Result []ScriptHashHistoryItem `json:"result"`

	// Error is an error message if the request failed, otherwise it is empty.
	Error string `json:"error,omitempty"`
}

// ScriptHashHistoryItem represents a single entry in the script hash history response.
type ScriptHashHistoryItem struct {
	// TxID is the transaction ID associated with the script hash history entry.
	TxID string `json:"tx_hash"`

	// Height is the block height at which the transaction was included (optional for unconfirmed)
	Height *int `json:"height,omitempty"`
}

// ScriptHistoryOrder defines the order for script history results
type ScriptHistoryOrder string

const (
	// ScriptHistoryOrderAsc is used to sort script history results in ascending order
	ScriptHistoryOrderAsc ScriptHistoryOrder = "asc"
	// ScriptHistoryOrderDesc is the default order for script history results
	ScriptHistoryOrderDesc ScriptHistoryOrder = "desc" // Default
)

// GetConfirmedScriptHistoryOpts defines options for retrieving confirmed script history
type GetConfirmedScriptHistoryOpts struct {
	// Order specifies the order of results, either ascending or descending
	Order *ScriptHistoryOrder
	// Limit specifies the maximum number of results to return (1-1000)
	Limit *int
	// Height filters results to those confirmed at or after the specified block height
	Height *int
	// NextPageToken is used for pagination, allowing retrieval of the next set of results
	NextPageToken *string
}

// Validate checks if the order value is valid
func (o ScriptHistoryOrder) Validate() error {
	switch o {
	case ScriptHistoryOrderAsc, ScriptHistoryOrderDesc:
		return nil
	default:
		return fmt.Errorf("invalid order '%s': must be 'asc' or 'desc'", o)
	}
}

// String returns the string representation of the order
func (o ScriptHistoryOrder) String() string {
	return string(o)
}

// Validate validates all the options and returns an error if any are invalid
func (opts *GetConfirmedScriptHistoryOpts) Validate() error {
	if opts == nil {
		return nil
	}

	if opts.Order != nil {
		if err := opts.Order.Validate(); err != nil {
			return fmt.Errorf("invalid order: %w", err)
		}
	}

	if opts.Limit != nil {
		if *opts.Limit < 1 || *opts.Limit > 1000 {
			return fmt.Errorf("invalid limit %d: must be between 1 and 1000", *opts.Limit)
		}
	}

	if opts.Height != nil {
		if *opts.Height < 0 {
			return fmt.Errorf("invalid height %d: must be 0 or above", *opts.Height)
		}
	}

	return nil
}

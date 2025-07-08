package wdk

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"

// SendWithResultStatus represents the status of a sending operation with a result.
type SendWithResultStatus string

// Possible values for SendWithResultStatus
const (
	SendWithResultStatusUnproven SendWithResultStatus = "unproven"
	SendWithResultStatusSending  SendWithResultStatus = "sending"
	SendWithResultStatusFailed   SendWithResultStatus = "failed"
)

// ReviewActionResultStatus represents the status of a reviewed action, describing the result of the review process.
type ReviewActionResultStatus string

// Possible values for ReviewActionResultStatus
const (
	ReviewActionResultStatusSuccess      ReviewActionResultStatus = "success"
	ReviewActionResultStatusDoubleSpend  ReviewActionResultStatus = "doubleSpend"
	ReviewActionResultStatusServiceError ReviewActionResultStatus = "serviceError"
	ReviewActionResultStatusInvalidTx    ReviewActionResultStatus = "invalidTx"
)

// SendWithResult represents the result of a send operation, including the transaction ID and the status of the operation.
type SendWithResult struct {
	TxID   primitives.TXIDHexString `json:"txid"`
	Status SendWithResultStatus     `json:"status"`
}

// ReviewActionResult represents the outcome of a review action, including transaction ID, status, and competing data.
type ReviewActionResult struct {
	TxID          primitives.TXIDHexString     `json:"txid"`
	Status        ReviewActionResultStatus     `json:"status"`
	CompetingTxs  []string                     `json:"competingTxs,omitempty"`
	CompetingBeef primitives.ExplicitByteArray `json:"competingBeef,omitempty"`
}

// ProcessActionResult represents the result of processing an action, including send results, non-delayed results, and a log.
type ProcessActionResult struct {
	SendWithResults   []SendWithResult     `json:"sendWithResults,omitempty"`
	NotDelayedResults []ReviewActionResult `json:"notDelayedResults,omitempty"`
	Log               *string              `json:"log,omitempty"`
}

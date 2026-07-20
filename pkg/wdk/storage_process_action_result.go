package wdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// SendWithResultStatus represents the status of a sending operation with a result.
type SendWithResultStatus string

// Possible values for SendWithResultStatus
const (
	SendWithResultStatusUnproven SendWithResultStatus = "unproven"
	SendWithResultStatusSending  SendWithResultStatus = "sending"
	SendWithResultStatusFailed   SendWithResultStatus = "failed"
)

// ToStandardizedStatus returns standardized status of a transaction request based on its ProvenTxReqStatus.
func (s SendWithResultStatus) ToStandardizedStatus() StandardizedTxStatus {
	switch s {
	case SendWithResultStatusUnproven:
		return TxUpdateStatusBroadcasted
	case SendWithResultStatusSending:
		return TxUpdateStatusWaiting
	case SendWithResultStatusFailed:
		return TxUpdateStatusServiceError
	default:
		return TxUpdateStatusUnknown
	}
}

// ReviewActionResultStatus represents the status of a reviewed action, describing the result of the review process.
type ReviewActionResultStatus string

// Possible values for ReviewActionResultStatus
const (
	ReviewActionResultStatusSuccess      ReviewActionResultStatus = "success"
	ReviewActionResultStatusDoubleSpend  ReviewActionResultStatus = "doubleSpend"
	ReviewActionResultStatusServiceError ReviewActionResultStatus = "serviceError"
	ReviewActionResultStatusInvalidTx    ReviewActionResultStatus = "invalidTx"
)

// ToStandardizedStatus returns standardized status of a transaction request based on its ProvenTxReqStatus.
func (s ReviewActionResultStatus) ToStandardizedStatus() StandardizedTxStatus {
	switch s {
	case ReviewActionResultStatusSuccess:
		return TxUpdateStatusBroadcasted
	case ReviewActionResultStatusDoubleSpend:
		return TxUpdateStatusDoubleSpend
	case ReviewActionResultStatusInvalidTx:
		return TxUpdateStatusInvalidTx
	case ReviewActionResultStatusServiceError:
		return TxUpdateStatusServiceError
	default:
		return TxUpdateStatusUnknown
	}
}

// SendWithResult represents the result of a send operation, including the transaction ID and the status of the operation.
type SendWithResult struct {
	TxID   primitives.TXIDHexString `json:"txid"`
	Status SendWithResultStatus     `json:"status"`
}

// ReviewActionErrors maps service/component names to errors for a review outcome.
// Wire format is map[string]string (error messages) so remote storage JSON can round-trip.
type ReviewActionErrors map[string]error

// MarshalJSON encodes errors as a map of service name → message string.
func (e ReviewActionErrors) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte("null"), nil
	}
	out := make(map[string]string, len(e))
	for k, v := range e {
		if v != nil {
			out[k] = v.Error()
		} else {
			out[k] = ""
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal review action errors: %w", err)
	}
	return b, nil
}

// UnmarshalJSON decodes a map of service name → message string into errors.
// Also accepts the legacy empty-object form {"svc":{}} produced by encoding/json
// for bare map[string]error values before this type existed.
func (e *ReviewActionErrors) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*e = nil
		return nil
	}

	var asStrings map[string]string
	if err := json.Unmarshal(data, &asStrings); err == nil {
		out := make(ReviewActionErrors, len(asStrings))
		for k, v := range asStrings {
			out[k] = errors.New(v)
		}
		*e = out
		return nil
	}

	// Legacy: map[string]error marshaled as {"name":{}} (empty objects).
	var asObjects map[string]json.RawMessage
	if err := json.Unmarshal(data, &asObjects); err != nil {
		return fmt.Errorf("unmarshal review action errors: %w", err)
	}
	out := make(ReviewActionErrors, len(asObjects))
	for k := range asObjects {
		out[k] = errors.New("")
	}
	*e = out
	return nil
}

// ReviewActionResult represents the outcome of a review action, including transaction ID, status, and competing data.
type ReviewActionResult struct {
	TxID          primitives.TXIDHexString     `json:"txid"`
	Status        ReviewActionResultStatus     `json:"status"`
	CompetingTxs  []string                     `json:"competingTxs,omitempty"`
	CompetingBeef primitives.ExplicitByteArray `json:"competingBeef,omitempty"`
	Reference     string                       `json:"reference,omitempty"`
	Labels        []string                     `json:"labels,omitempty"`
	Errors        ReviewActionErrors           `json:"errors,omitempty"`
}

// ProcessActionResult represents the result of processing an action, including send results, non-delayed results, and a log.
type ProcessActionResult struct {
	SendWithResults   []SendWithResult     `json:"sendWithResults,omitempty"`
	NotDelayedResults []ReviewActionResult `json:"notDelayedResults,omitempty"`
	Log               *string              `json:"log,omitempty"`
}

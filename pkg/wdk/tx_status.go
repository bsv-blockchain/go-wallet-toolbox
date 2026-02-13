package wdk

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// TxStatus Transaction status stored in database
type TxStatus string

// Possible transaction statuses stored in database
const (
	TxStatusCompleted   TxStatus = "completed"
	TxStatusFailed      TxStatus = "failed"
	TxStatusUnprocessed TxStatus = "unprocessed"
	TxStatusSending     TxStatus = "sending"
	TxStatusUnproven    TxStatus = "unproven"
	TxStatusUnsigned    TxStatus = "unsigned"
	TxStatusNoSend      TxStatus = "nosend"
	TxStatusNonFinal    TxStatus = "nonfinal"
	TxStatusUnfail      TxStatus = "unfail"
)

// String returns the string representation of the TxStatus.
func (s TxStatus) String() string {
	return string(s)
}

// ToUTXOStatus converts a TxStatus value to its corresponding UTXOStatus based on predefined status mappings.
func (s TxStatus) ToUTXOStatus() UTXOStatus {
	switch s { //nolint:exhaustive
	case TxStatusCompleted:
		return UTXOStatusMined
	case TxStatusSending:
		return UTXOStatusSending
	case TxStatusUnproven:
		return UTXOStatusUnproven
	default:
		return UTXOStatusUnknown
	}
}

// ToStandardizedStatus returns standardized status of a transaction request based on its ProvenTxReqStatus.
func (s TxStatus) ToStandardizedStatus() StandardizedTxStatus {
	switch s {
	case TxStatusCompleted:
		return TxUpdateStatusMined
	case TxStatusUnproven:
		return TxUpdateStatusBroadcasted
	case TxStatusSending, TxStatusUnprocessed, TxStatusNoSend, TxStatusNonFinal, TxStatusUnsigned, TxStatusUnfail:
		return TxUpdateStatusWaiting
	case TxStatusFailed:
		return TxUpdateStatusInvalidTx
	default:
		return TxUpdateStatusUnknown
	}
}

// ProvenTxReqStatus represents the status of a proven transaction in a defined processing state as a string.
type ProvenTxReqStatus string

// Possible proven transaction statuses stored in database
const (
	ProvenTxStatusSending     ProvenTxReqStatus = "sending"
	ProvenTxStatusUnsent      ProvenTxReqStatus = "unsent"
	ProvenTxStatusNoSend      ProvenTxReqStatus = "nosend"
	ProvenTxStatusUnknown     ProvenTxReqStatus = "unknown"
	ProvenTxStatusNonFinal    ProvenTxReqStatus = "nonfinal"
	ProvenTxStatusUnprocessed ProvenTxReqStatus = "unprocessed"
	ProvenTxStatusUnmined     ProvenTxReqStatus = "unmined"
	ProvenTxStatusCallback    ProvenTxReqStatus = "callback"
	ProvenTxStatusUnconfirmed ProvenTxReqStatus = "unconfirmed"
	ProvenTxStatusCompleted   ProvenTxReqStatus = "completed"
	ProvenTxStatusInvalid     ProvenTxReqStatus = "invalidTx"
	ProvenTxStatusDoubleSpend ProvenTxReqStatus = "doubleSpend"
	ProvenTxStatusUnfail      ProvenTxReqStatus = "unfail"
	ProvenTxStatusReorg       ProvenTxReqStatus = "reorg"
)

// SendWithResultStatus returns the status of a transaction request based on its ProvenTxReqStatus.
func (s ProvenTxReqStatus) SendWithResultStatus() SendWithResultStatus {
	if s.Sending() {
		return SendWithResultStatusSending
	}

	if s.AlreadySent() {
		return SendWithResultStatusUnproven
	}

	return SendWithResultStatusFailed
}

// Sending returns true if the ProvenTxReqStatus is considered still in the sending or processing phase.
func (s ProvenTxReqStatus) Sending() bool {
	switch s { //nolint:exhaustive
	case ProvenTxStatusUnknown,
		ProvenTxStatusNonFinal,
		ProvenTxStatusInvalid,
		ProvenTxStatusDoubleSpend,
		ProvenTxStatusSending,
		ProvenTxStatusUnsent,
		ProvenTxStatusNoSend,
		ProvenTxStatusUnprocessed:
		return true
	default:
		return false
	}
}

// AlreadySent returns true if the transaction status indicates it has already been sent or processed.
func (s ProvenTxReqStatus) AlreadySent() bool {
	switch s { //nolint:exhaustive
	case ProvenTxStatusUnmined,
		ProvenTxStatusCallback,
		ProvenTxStatusUnconfirmed,
		ProvenTxStatusCompleted,
		ProvenTxStatusReorg:
		return true
	default:
		return false
	}
}

// ToStandardizedStatus returns standardized status of a transaction request based on its ProvenTxReqStatus.
func (s ProvenTxReqStatus) ToStandardizedStatus() StandardizedTxStatus {
	switch s {
	case ProvenTxStatusCompleted:
		return TxUpdateStatusMined
	case ProvenTxStatusUnmined, ProvenTxStatusCallback, ProvenTxStatusUnconfirmed:
		return TxUpdateStatusBroadcasted
	case ProvenTxStatusSending, ProvenTxStatusUnsent, ProvenTxStatusUnprocessed, ProvenTxStatusNoSend, ProvenTxStatusNonFinal, ProvenTxStatusUnfail, ProvenTxStatusReorg:
		return TxUpdateStatusWaiting
	case ProvenTxStatusInvalid:
		return TxUpdateStatusInvalidTx
	case ProvenTxStatusDoubleSpend:
		return TxUpdateStatusDoubleSpend
	case ProvenTxStatusUnknown:
		return TxUpdateStatusUnknown
	default:
		return TxUpdateStatusUnknown
	}
}

// ProvenTxReqProblematicStatuses contains transaction statuses considered problematic, such as unknown, nonfinal, invalid, and double spend.
var ProvenTxReqProblematicStatuses = []ProvenTxReqStatus{
	ProvenTxStatusUnknown,
	ProvenTxStatusNonFinal,
	ProvenTxStatusInvalid,
	ProvenTxStatusDoubleSpend,
}

// ProvenTxReqBeyondBroadcastStageStatuses contains statuses indicating a proven transaction has passed the broadcast stage.
var ProvenTxReqBeyondBroadcastStageStatuses = []ProvenTxReqStatus{
	ProvenTxStatusUnmined,
	ProvenTxStatusCompleted,
}

// CurrentTxStatus represents the response from a monitoring task
type CurrentTxStatus struct {
	TxID        string
	Status      StandardizedTxStatus
	BlockHash   string
	BlockHeight uint32
	MerklePath  *transaction.MerklePath
	MerkleRoot  string
	Error       *CurrentTxError
	Reference   string
}

// CurrentTxError represents the error details for a transaction status update, including competing transactions and error messages.
type CurrentTxError struct {
	CompetingTxs []string         // only when double spend is detected, list of competing txids
	Errors       map[string]error // error message describing the issue, e.g. "double spend detected", "transaction invalid", etc.
}

// StandardizedTxStatus represents the status of a transaction in a monitoring task response
type StandardizedTxStatus string

// Possible values for StandardizedTxStatus
const (
	TxUpdateStatusBroadcasted  StandardizedTxStatus = "broadcasted"
	TxUpdateStatusDoubleSpend  StandardizedTxStatus = "doubleSpend"
	TxUpdateStatusInvalidTx    StandardizedTxStatus = "invalidTx"
	TxUpdateStatusServiceError StandardizedTxStatus = "serviceError"
	TxUpdateStatusWaiting      StandardizedTxStatus = "waiting"
	TxUpdateStatusMined        StandardizedTxStatus = "mined"
	TxUpdateStatusUnknown      StandardizedTxStatus = "unknown"
)

// String returns the string representation of StandardizedTxStatus
func (s StandardizedTxStatus) String() string {
	return string(s)
}

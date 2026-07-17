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
	switch s { //nolint:exhaustive // default case handles remaining statuses
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
// Terminal failures (invalidTx, doubleSpend) are intentionally excluded: they are not "still sending",
// so SendWithResultStatus() correctly falls through to failed for them.
func (s ProvenTxReqStatus) Sending() bool {
	switch s { //nolint:exhaustive // default case handles remaining statuses
	case ProvenTxStatusUnknown,
		ProvenTxStatusNonFinal,
		ProvenTxStatusSending,
		ProvenTxStatusUnsent,
		ProvenTxStatusNoSend,
		ProvenTxStatusUnprocessed:
		return true
	default:
		return false
	}
}

// AlreadySent returns true if the transaction status is evidence the network currently accepts it.
// reorg is intentionally excluded: a reorged tx has had its proof invalidated and is NOT currently
// accepted, so it must not be treated as already-sent (that would pretend network acceptance during
// internalize/broadcast). Reorg re-sync eligibility rides on WasBroadcastStatus(), not this predicate.
func (s ProvenTxReqStatus) AlreadySent() bool {
	switch s { //nolint:exhaustive // default case handles remaining statuses
	case ProvenTxStatusUnmined,
		ProvenTxStatusCallback,
		ProvenTxStatusUnconfirmed,
		ProvenTxStatusCompleted:
		return true
	default:
		return false
	}
}

// IsInFlight returns true while the tx is being pushed toward the network but has no acceptance
// evidence yet: {sending, unsent, unprocessed}.
func (s ProvenTxReqStatus) IsInFlight() bool {
	switch s { //nolint:exhaustive // default case handles remaining statuses
	case ProvenTxStatusSending,
		ProvenTxStatusUnsent,
		ProvenTxStatusUnprocessed:
		return true
	default:
		return false
	}
}

// IsTerminalFailure returns true for statuses that are permanent, unrecoverable failures:
// {invalidTx, doubleSpend}.
func (s ProvenTxReqStatus) IsTerminalFailure() bool {
	switch s { //nolint:exhaustive // default case handles remaining statuses
	case ProvenTxStatusInvalid,
		ProvenTxStatusDoubleSpend:
		return true
	default:
		return false
	}
}

// IsAcceptedUnproven returns true for statuses proving network acceptance but without a mined proof yet:
// {unmined, callback, unconfirmed}. Distinct from IsFinalMined (completed).
func (s ProvenTxReqStatus) IsAcceptedUnproven() bool {
	switch s { //nolint:exhaustive // default case handles remaining statuses
	case ProvenTxStatusUnmined,
		ProvenTxStatusCallback,
		ProvenTxStatusUnconfirmed:
		return true
	default:
		return false
	}
}

// IsFinalMined returns true only for the terminal mined-and-proven status: {completed}.
func (s ProvenTxReqStatus) IsFinalMined() bool {
	return s == ProvenTxStatusCompleted
}

// WasBroadcastStatus returns true for states proving the transaction was accepted by the network.
func (s ProvenTxReqStatus) WasBroadcastStatus() bool {
	switch s { //nolint:exhaustive // default case handles remaining statuses
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
	ProvenTxStatusCallback,
	ProvenTxStatusUnconfirmed,
	ProvenTxStatusCompleted,
	ProvenTxStatusReorg,
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
	Labels      []string
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

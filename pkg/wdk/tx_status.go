package wdk

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
	ProvenTxStatusInvalid     ProvenTxReqStatus = "invalid"
	ProvenTxStatusDoubleSpend ProvenTxReqStatus = "doubleSpend"
	ProvenTxStatusUnfail      ProvenTxReqStatus = "unfail"
)

// TxReqBroadcastStatus is a reduced ProvenTxReqStatus, used to decide whether to broadcast a transaction or not.
type TxReqBroadcastStatus string

// Possible transaction request broadcast statuses
const (
	TxReqBroadcastReadyToSend TxReqBroadcastStatus = "readyToSend"
	TxReqBroadcastAlreadySent TxReqBroadcastStatus = "alreadySent"
	TxReqBroadcastError       TxReqBroadcastStatus = "error"
	TxReqBroadcastUnknown     TxReqBroadcastStatus = "unknown"
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

func (s ProvenTxReqStatus) AlreadySent() bool {
	switch s { //nolint:exhaustive
	case ProvenTxStatusUnmined,
		ProvenTxStatusCallback,
		ProvenTxStatusUnconfirmed,
		ProvenTxStatusCompleted:
		return true
	default:
		return false
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

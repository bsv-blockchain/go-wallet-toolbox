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

// BroadcastStatus returns the simplified broadcast status of a transaction request based on its current status.
func (s ProvenTxReqStatus) BroadcastStatus() TxReqBroadcastStatus {
	switch s {
	case ProvenTxStatusUnknown,
		ProvenTxStatusNonFinal,
		ProvenTxStatusInvalid,
		ProvenTxStatusDoubleSpend:
		return TxReqBroadcastError
	case ProvenTxStatusSending,
		ProvenTxStatusUnsent,
		ProvenTxStatusNoSend,
		ProvenTxStatusUnprocessed:
		return TxReqBroadcastReadyToSend
	case ProvenTxStatusUnmined,
		ProvenTxStatusCallback,
		ProvenTxStatusUnconfirmed,
		ProvenTxStatusCompleted:
		return TxReqBroadcastAlreadySent
	case ProvenTxStatusUnfail:
		fallthrough
	default:
		return TxReqBroadcastUnknown
	}
}

// ProvenTxReqStatusesForSourceTransactions is a provenTxReq status list of txs that can be taken as subject-transaction's input sources
var ProvenTxReqStatusesForSourceTransactions = []ProvenTxReqStatus{
	ProvenTxStatusUnsent,
	ProvenTxStatusUnmined,
	ProvenTxStatusUnconfirmed,
	ProvenTxStatusSending,
	ProvenTxStatusNoSend,
	ProvenTxStatusCompleted,
}

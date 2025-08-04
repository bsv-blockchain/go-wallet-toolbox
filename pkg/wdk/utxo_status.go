package wdk

// UTXOStatus represents the current state of an unspent transaction output within the transaction processing lifecycle.
type UTXOStatus string

// Possible UTXO statuses.
const (
	UTXOStatusSending  UTXOStatus = "sending"
	UTXOStatusUnproven UTXOStatus = "unproven"
	UTXOStatusMined    UTXOStatus = "mined"

	UTXOStatusUnknown = ""
)

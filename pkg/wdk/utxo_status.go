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

// BoundedUTXOQuery bundles the stable selection filters shared by the bounded funder UTXO
// queries — the owning user and basket, the status tier being scanned, and the output IDs
// already allocated in this funding pass that must be skipped. Grouping them keeps the
// repository query signatures below the parameter-count limit and lets callers build the
// filter once per tier.
type BoundedUTXOQuery struct {
	UserID            int
	BasketName        string
	Status            UTXOStatus
	ExcludedOutputIDs []uint
}

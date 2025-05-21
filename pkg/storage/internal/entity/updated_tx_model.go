package entity

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
)

type UpdatedTx struct {
	UserID        int
	TransactionID uint
	TxID          string
	TxStatus      wdk.TxStatus
	ReqTxStatus   wdk.ProvenTxReqStatus
	InputBeef     []byte
	RawTx         []byte
	Tx            *transaction.Transaction
}

func (ut *UpdatedTx) GetLockingScript(vout uint32) (*string, error) {
	if ut.Tx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}
	voutInt, err := to.IntFromUnsigned(vout)
	if err != nil {
		return nil, fmt.Errorf("failed to convert vout to int: %w", err)
	}

	if voutInt >= len(ut.Tx.Outputs) {
		return nil, fmt.Errorf("output index %d is out of range of provided tx outputs count %d", vout, len(ut.Tx.Outputs))
	}
	return to.Ptr(ut.Tx.Outputs[voutInt].LockingScript.String()), nil
}

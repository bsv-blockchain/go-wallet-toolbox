package entity

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
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

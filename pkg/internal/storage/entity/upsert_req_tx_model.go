package entity

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type UpsertKnownTx struct {
	InputBeef     []byte
	RawTx         []byte
	TxID          string
	Status        wdk.ProvenTxReqStatus
	SkipForStatus *wdk.ProvenTxReqStatus
}

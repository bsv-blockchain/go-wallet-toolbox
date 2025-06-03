package entity

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type UpsertProvenTxReq struct {
	InputBeef     []byte
	RawTx         []byte
	TxID          string
	Status        wdk.ProvenTxReqStatus
	SkipForStatus *wdk.ProvenTxReqStatus
}

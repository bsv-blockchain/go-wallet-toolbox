package entity

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type UpsertProvenTxReq struct {
	InputBeef       []byte
	RawTx           []byte
	TxID            string
	Status          wdk.ProvenTxReqStatus
	SkipForStatuses []wdk.ProvenTxReqStatus
}

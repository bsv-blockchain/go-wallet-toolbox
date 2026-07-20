package models

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type ProvenTxReq struct {
	Timestamps

	ProvenTxReqID       uint                  `gorm:"column:provenTxReqId;primaryKey;autoIncrement"`
	ProvenTxID          *uint                 `gorm:"column:provenTxId"`
	Status              wdk.ProvenTxReqStatus `gorm:"column:status;default:unknown"`
	Attempts            uint64                `gorm:"column:attempts"`
	RebroadcastAttempts int                   `gorm:"column:rebroadcastAttempts;default:0"`
	Notified            bool                  `gorm:"column:notified"`
	Batch               *string               `gorm:"column:batch;index"`
	WasBroadcast        bool                  `gorm:"column:wasBroadcast"`
	TxID                string                `gorm:"column:txid;type:varchar(64);not null;uniqueIndex"`
	RawTx               []byte                `gorm:"column:rawTx;type:binary;not null"`
	InputBeef           []byte                `gorm:"column:inputBeef"`
	History             string                `gorm:"column:history;type:text;not null;default:'{}'"`
	Notify              string                `gorm:"column:notify;type:text;not null;default:'{}'"`

	ProvenTx *ProvenTx `gorm:"foreignKey:ProvenTxID"`
}

func (ProvenTxReq) TableName() string {
	return "bsv_proven_tx_reqs"
}

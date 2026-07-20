package models

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Transaction struct {
	Timestamps

	TransactionID uint         `gorm:"column:transactionId;primaryKey;autoIncrement"`
	UserID        int          `gorm:"column:userId"`
	Status        wdk.TxStatus `gorm:"column:status"`
	Reference     string       `gorm:"column:reference;uniqueIndex"`
	IsOutgoing    bool         `gorm:"column:isOutgoing"`
	Satoshis      int64        `gorm:"column:satoshis"`
	Description   string       `gorm:"column:description;type:varchar(2048)"`
	Version       *uint32      `gorm:"column:version"`
	LockTime      *uint32      `gorm:"column:lockTime"`
	TxID          *string      `gorm:"column:txid;index"`
	InputBeef     []byte       `gorm:"column:inputBeef"`
	ProvenTxID    *uint        `gorm:"column:provenTxId"`
	RawTx         []byte       `gorm:"column:rawTx"`

	Outputs    []*Output   `gorm:"foreignKey:TransactionID;references:TransactionID"`
	Inputs     []*Output   `gorm:"foreignKey:SpentBy;references:TransactionID"`
	Labels     []*TxLabel  `gorm:"many2many:tx_labels_map;joinForeignKey:TransactionID;joinReferences:TxLabelID"`
	Commission *Commission `gorm:"foreignKey:TransactionID"`
	ProvenTx   *ProvenTx   `gorm:"foreignKey:ProvenTxID"`
}

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
	// Labels is populated/persisted manually via bsv_tx_labels_map (see repo.Transactions) rather than
	// through a GORM many2many association: GORM's automatic join-table column derivation always
	// strips explicit `column:` tags and re-derives snake_case names, which cannot represent this
	// table's required camelCase FK columns (transactionId/txLabelId per target-schema.md).
	Labels []*TxLabel `gorm:"-"`
	Commission *Commission `gorm:"foreignKey:TransactionID"`
	ProvenTx   *ProvenTx   `gorm:"foreignKey:ProvenTxID"`
}

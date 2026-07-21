package models

type TxLabelsMap struct {
	Timestamps
	TxLabelID     uint `gorm:"column:txLabelId;primaryKey"`
	TransactionID uint `gorm:"column:transactionId;primaryKey;index"`
	IsDeleted     bool `gorm:"column:isDeleted;default:false"`
}

func (TxLabelsMap) TableName() string {
	return "bsv_tx_labels_map"
}

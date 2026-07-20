package models

type TxLabelsMap struct {
	Timestamps
	TxLabelID     uint `gorm:"column:tx_label_id;primaryKey"`
	TransactionID uint `gorm:"column:transaction_id;primaryKey"`
	IsDeleted     bool `gorm:"column:isDeleted;default:false"`
}

func (TxLabelsMap) TableName() string {
	return "bsv_tx_labels_map"
}

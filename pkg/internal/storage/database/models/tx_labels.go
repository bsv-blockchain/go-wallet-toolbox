package models

type TxLabel struct {
	Timestamps
	TxLabelID uint   `gorm:"primarykey;column:txLabelId"`
	Label     string `gorm:"type:varchar(300);column:label;uniqueIndex:idx_tx_labels_label_user"`
	UserID    int    `gorm:"column:userId;uniqueIndex:idx_tx_labels_label_user"`
	IsDeleted bool   `gorm:"column:isDeleted;default:false"`
}

func (TxLabel) TableName() string {
	return "bsv_tx_labels"
}

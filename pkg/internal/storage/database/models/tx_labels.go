package models

type TxLabel struct {
	Timestamps
	TxLabelID uint   `gorm:"primarykey;column:txLabelId"`
	Label     string `gorm:"type:varchar(300);column:label"`
	UserID    int    `gorm:"column:userId"`
	IsDeleted bool   `gorm:"column:isDeleted;default:false"`

	Transactions []*Transaction `gorm:"many2many:tx_labels_map;joinForeignKey:TxLabelID;joinReferences:TransactionID"`
}

func (TxLabel) TableName() string {
	return "bsv_tx_labels"
}

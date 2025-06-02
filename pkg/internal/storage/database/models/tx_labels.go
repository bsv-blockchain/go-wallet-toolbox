package models

type TransactionLabels struct {
	TransactionID uint   `gorm:"column:transaction_id"`
	LabelName     string `gorm:"column:label_name"`
	LabelUserID   int    `gorm:"column:label_user_id"`
}

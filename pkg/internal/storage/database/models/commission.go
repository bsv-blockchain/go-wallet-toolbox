package models

type Commission struct {
	Timestamps

	CommissionID  uint   `gorm:"column:commissionId;primaryKey;autoIncrement"`
	UserID        int    `gorm:"column:userId;not null;uniqueIndex:idx_commission_user_tx"`
	TransactionID uint   `gorm:"column:transactionId;not null;uniqueIndex:idx_commission_user_tx"`
	Satoshis      int    `gorm:"column:satoshis;not null"`
	KeyOffset     string `gorm:"column:keyOffset;type:varchar(130)"`
	IsRedeemed    bool   `gorm:"column:isRedeemed;not null;default:false"`
	LockingScript []byte `gorm:"column:lockingScript;not null"`
}

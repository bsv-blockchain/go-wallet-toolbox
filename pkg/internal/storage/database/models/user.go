package models

// User is the database model of the user
type User struct {
	Timestamps

	UserID        int    `gorm:"column:userId;primaryKey;autoIncrement"`
	IdentityKey   string `gorm:"column:identityKey;type:varchar(130);not null;default:'';uniqueIndex"`
	ActiveStorage string `gorm:"column:activeStorage;type:varchar(130);not null;default:''"`

	OutputBaskets     []*OutputBasket     `gorm:"foreignKey:UserID"`
	Certificates      []*Certificate      `gorm:"foreignKey:UserID"`
	CertificateFields []*CertificateField `gorm:"foreignKey:UserID"`
}

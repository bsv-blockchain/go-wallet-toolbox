package models

import "time"

// User is the database model of the user
type User struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	UserID        int    `gorm:"primaryKey;not null"`
	IdentityKey   string `gorm:"type:varchar(130);not null"`
	ActiveStorage string `gorm:"type:varchar(255);not null"`

	OutputBaskets     []*OutputBasket     `gorm:"foreignKey:UserID"`
	Certificates      []*Certificate      `gorm:"foreignKey:UserID"`
	CertificateFields []*CertificateField `gorm:"foreignKey:UserID"`
}

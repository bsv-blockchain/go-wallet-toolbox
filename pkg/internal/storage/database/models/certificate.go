package models

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Certificate is the database model of the certificate
type Certificate struct {
	gorm.Model

	Type               string `gorm:"type:varchar(100);not null;uniqueIndex:idx_certifier_type_serial_number_user_id"`
	SerialNumber       string `gorm:"type:varchar(100);not null;uniqueIndex:idx_certifier_type_serial_number_user_id"`
	Certifier          string `gorm:"type:varchar(100);not null;uniqueIndex:idx_certifier_type_serial_number_user_id"`
	Subject            string `gorm:"type:varchar(100);not null"`
	Verifier           string `gorm:"type:varchar(100)"`
	RevocationOutpoint string `gorm:"type:varchar(100);not null"`
	Signature          string `gorm:"type:varchar(255);not null"`

	UserID            int                 `gorm:"uniqueIndex:idx_certifier_type_serial_number_user_id"`
	CertificateFields []*CertificateField `gorm:"foreignKey:CertificateID"`
}

// CertificateField is a database model of the fields related to Certificate
type CertificateField struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	FieldName  string `gorm:"type:varchar(100);not null;uniqueIndex:idx_field_name_certificate_id"`
	FieldValue string `gorm:"type:varchar(100);not null"`
	MasterKey  string `gorm:"type:varchar(255);not null"`

	UserID        int
	CertificateID uint `gorm:"uniqueIndex:idx_field_name_certificate_id"`
}

func (cf *CertificateField) BeforeCreate(tx *gorm.DB) error {
	tx.Statement.AddClause(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "field_name"},
			{Name: "certificate_id"},
		},
		DoNothing: true,
	})

	return nil
}

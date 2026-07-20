package models

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Certificate is the database model of the certificate
type Certificate struct {
	Timestamps

	CertificateID      uint   `gorm:"column:certificateId;primaryKey;autoIncrement"`
	Type               string `gorm:"column:type;type:varchar(100);not null;uniqueIndex:idx_certifier_type_serial_number_user_id"`
	SerialNumber       string `gorm:"column:serialNumber;type:varchar(100);not null;uniqueIndex:idx_certifier_type_serial_number_user_id"`
	Certifier          string `gorm:"column:certifier;type:varchar(100);not null;uniqueIndex:idx_certifier_type_serial_number_user_id"`
	Subject            string `gorm:"column:subject;type:varchar(100);not null"`
	Verifier           string `gorm:"column:verifier;type:varchar(100)"`
	RevocationOutpoint string `gorm:"column:revocationOutpoint;type:varchar(100);not null"`
	Signature          string `gorm:"column:signature;type:varchar(255);not null"`
	IsDeleted          bool   `gorm:"column:isDeleted;default:false"`

	UserID            int                 `gorm:"column:userId;uniqueIndex:idx_certifier_type_serial_number_user_id"`
	CertificateFields []*CertificateField `gorm:"foreignKey:CertificateID"`
}

// CertificateField is a database model of the fields related to Certificate
type CertificateField struct {
	Timestamps

	FieldName  string `gorm:"column:fieldName;type:varchar(100);not null;uniqueIndex:idx_field_name_certificate_id"`
	FieldValue string `gorm:"column:fieldValue;type:varchar(255);not null"`
	MasterKey  string `gorm:"column:masterKey;type:varchar(255);not null;default:''"`

	UserID        int  `gorm:"column:userId"`
	CertificateID uint `gorm:"column:certificateId;uniqueIndex:idx_field_name_certificate_id"`
}

func (cf *CertificateField) BeforeCreate(tx *gorm.DB) error {
	tx.Statement.AddClause(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "fieldName"},
			{Name: "certificateId"},
		},
		DoNothing: true,
	})

	return nil
}

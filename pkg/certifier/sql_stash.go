package certifier

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

const sqliteInMemoryDSN = ":memory:"

// SQLStash is a CertificateStash backed by a SQL database through GORM.
// It uses its own tables and is meant to live in a database separate from the wallet storage.
type SQLStash struct {
	db    *gorm.DB
	owned bool
}

// NewSQLiteStash opens a SQLite database for the stash and creates its tables.
// An empty dsn opens an in-memory database, which is lost when the stash is closed.
func NewSQLiteStash(dsn string) (*SQLStash, error) {
	inMemory := dsn == "" || dsn == sqliteInMemoryDSN
	if inMemory {
		dsn = sqliteInMemoryDSN
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: glogger.Discard})
	if err != nil {
		return nil, fmt.Errorf("failed to open certificate stash database: %w", err)
	}

	if inMemory {
		// every connection to ":memory:" gets its own empty database, so keep a single one alive
		sqlDB, dbErr := db.DB()
		if dbErr != nil {
			return nil, fmt.Errorf("failed to get certificate stash database connection: %w", dbErr)
		}
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetConnMaxLifetime(0)
		sqlDB.SetConnMaxIdleTime(0)
	}

	stash, err := NewSQLStash(db)
	if err != nil {
		closeGormDB(db)
		return nil, err
	}
	stash.owned = true
	return stash, nil
}

// NewSQLStash creates a stash on an existing GORM connection and migrates its tables.
// The caller stays responsible for closing the connection.
func NewSQLStash(db *gorm.DB) (*SQLStash, error) {
	if err := db.AutoMigrate(&issuedCertificateModel{}, &issuedCertificateFieldModel{}); err != nil {
		return nil, fmt.Errorf("failed to migrate certificate stash tables: %w", err)
	}
	return &SQLStash{db: db}, nil
}

// Close closes the database connection if it was opened by NewSQLiteStash.
func (s *SQLStash) Close() error {
	if !s.owned {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get certificate stash database connection: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close certificate stash database: %w", err)
	}
	return nil
}

// Save stores the certificate and its fields in a single transaction.
func (s *SQLStash) Save(ctx context.Context, cert *IssuedCertificate) error {
	model := toIssuedCertificateModel(cert)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(model).Error
	})
	if err != nil {
		return fmt.Errorf("failed to stash issued certificate: %w", err)
	}

	cert.ID = model.ID
	cert.CreatedAt = model.CreatedAt
	return nil
}

// FindBySerialNumber returns the certificate with the given serial number.
func (s *SQLStash) FindBySerialNumber(ctx context.Context, serialNumber string) (*IssuedCertificate, error) {
	return s.findOne(ctx, "serial_number = ?", serialNumber)
}

// FindByRevocationOutpoint returns the certificate with the given revocation outpoint.
func (s *SQLStash) FindByRevocationOutpoint(ctx context.Context, outpoint string) (*IssuedCertificate, error) {
	return s.findOne(ctx, "revocation_outpoint = ?", outpoint)
}

// FindBySubject returns all certificates issued to the given subject, oldest first.
func (s *SQLStash) FindBySubject(ctx context.Context, subject string) ([]*IssuedCertificate, error) {
	var models []*issuedCertificateModel
	err := s.db.WithContext(ctx).
		Preload("Fields", orderFieldsByName).
		Where("subject = ?", subject).
		Order("id").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find issued certificates by subject: %w", err)
	}

	certs := make([]*IssuedCertificate, 0, len(models))
	for _, m := range models {
		certs = append(certs, m.toIssuedCertificate())
	}
	return certs, nil
}

func (s *SQLStash) findOne(ctx context.Context, query string, arg any) (*IssuedCertificate, error) {
	var model issuedCertificateModel
	err := s.db.WithContext(ctx).Preload("Fields", orderFieldsByName).Where(query, arg).Take(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrIssuedCertificateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find issued certificate: %w", err)
	}
	return model.toIssuedCertificate(), nil
}

type issuedCertificateModel struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time

	Type               string `gorm:"type:varchar(100);not null"`
	SerialNumber       string `gorm:"type:varchar(100);not null;uniqueIndex"`
	Subject            string `gorm:"type:varchar(100);not null;index"`
	Certifier          string `gorm:"type:varchar(100);not null"`
	RevocationOutpoint string `gorm:"type:varchar(100);not null;index"`
	Signature          string `gorm:"type:varchar(255);not null"`
	RevocationTx       []byte

	Fields []*issuedCertificateFieldModel `gorm:"foreignKey:IssuedCertificateID;constraint:OnDelete:CASCADE"`
}

func (issuedCertificateModel) TableName() string {
	return "certifier_issued_certificates"
}

type issuedCertificateFieldModel struct {
	IssuedCertificateID uint   `gorm:"primaryKey"`
	FieldName           string `gorm:"type:varchar(100);primaryKey"`
	FieldValue          string `gorm:"type:text;not null"`
	MasterKey           string `gorm:"type:varchar(255)"`
}

func (issuedCertificateFieldModel) TableName() string {
	return "certifier_issued_certificate_fields"
}

func toIssuedCertificateModel(cert *IssuedCertificate) *issuedCertificateModel {
	fields := make([]*issuedCertificateFieldModel, 0, len(cert.Fields))
	for _, f := range cert.Fields {
		fields = append(fields, &issuedCertificateFieldModel{
			FieldName:  f.Name,
			FieldValue: f.Value,
			MasterKey:  f.MasterKey,
		})
	}

	return &issuedCertificateModel{
		Type:               cert.Type,
		SerialNumber:       cert.SerialNumber,
		Subject:            cert.Subject,
		Certifier:          cert.Certifier,
		RevocationOutpoint: cert.RevocationOutpoint,
		Signature:          cert.Signature,
		RevocationTx:       cert.RevocationTx,
		Fields:             fields,
	}
}

func (m issuedCertificateModel) toIssuedCertificate() *IssuedCertificate {
	fields := make([]IssuedCertificateField, 0, len(m.Fields))
	for _, f := range m.Fields {
		fields = append(fields, IssuedCertificateField{
			Name:      f.FieldName,
			Value:     f.FieldValue,
			MasterKey: f.MasterKey,
		})
	}

	return &IssuedCertificate{
		ID:                 m.ID,
		Type:               m.Type,
		SerialNumber:       m.SerialNumber,
		Subject:            m.Subject,
		Certifier:          m.Certifier,
		RevocationOutpoint: m.RevocationOutpoint,
		Signature:          m.Signature,
		Fields:             fields,
		RevocationTx:       m.RevocationTx,
		CreatedAt:          m.CreatedAt,
	}
}

func orderFieldsByName(db *gorm.DB) *gorm.DB {
	return db.Order("field_name")
}

func closeGormDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

package repo

import (
	"context"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/paging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
)

type Certificates struct {
	db *gorm.DB
}

type ListCertificatesActionParams struct {
	SerialNumber       *primitives.Base64String
	Subject            *primitives.PubKeyHex
	RevocationOutpoint *primitives.OutpointString
	Signature          *primitives.HexString
	Certifiers         []primitives.PubKeyHex
	Types              []primitives.Base64String
	Limit              primitives.PositiveIntegerDefault10Max10000
	Offset             primitives.PositiveInteger
}

func NewCertificates(db *gorm.DB) *Certificates {
	return &Certificates{db: db}
}

func (c *Certificates) CreateCertificate(ctx context.Context, certificate *models.Certificate) (uint, error) {
	err := c.db.WithContext(ctx).Create(certificate).Error
	if err != nil {
		return 0, fmt.Errorf("failed to create certificate model: %w", err)
	}
	return certificate.ID, nil
}

func (c *Certificates) DeleteCertificate(ctx context.Context, userID int, args wdk.RelinquishCertificateArgs) error {
	tx := c.db.WithContext(ctx).Delete(&models.Certificate{}, "type = ? AND serial_number = ? AND certifier = ? AND user_id = ?", args.Type, args.SerialNumber, args.Certifier, userID)
	if tx.RowsAffected == 0 {
		return fmt.Errorf("failed to delete certificate model: certificate not found")
	}
	if tx.Error != nil {
		return fmt.Errorf("failed to delete certificate model: %w", tx.Error)
	}

	return nil
}

func (c *Certificates) ListAndCountCertificates(ctx context.Context, userID int, opts ListCertificatesActionParams) (certificates []*models.Certificate, totalRows int64, err error) {
	err = c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		page := &paging.Page{}

		// parse offset and limit
		if opts.Limit > 0 {
			limit, err := to.IntFromUnsigned(opts.Limit)
			if err != nil {
				return fmt.Errorf("error during parsing limit: %w", err)
			}
			page.Limit = limit
		}

		if opts.Offset > 0 {
			ofs, err := to.IntFromUnsigned(opts.Offset)
			if err != nil {
				return fmt.Errorf("error during parsing offset: %w", err)
			}
			page.Offset = ofs
		}

		// prepare query
		query := tx.Model(&models.Certificate{}).Scopes(
			scopes.UserID(userID),
		)

		if opts.SerialNumber != nil {
			query = query.Where("serial_number = ?", opts.SerialNumber)
		}
		if opts.Subject != nil {
			query = query.Where("subject = ?", opts.Subject)
		}
		if opts.RevocationOutpoint != nil {
			query = query.Where("revocation_outpoint = ?", opts.RevocationOutpoint)
		}
		if opts.Signature != nil {
			query = query.Where("signature = ?", opts.Signature)
		}
		if len(opts.Certifiers) > 0 {
			query = query.Where("certifier IN ?", opts.Certifiers)
		}
		if len(opts.Types) > 0 {
			query = query.Where("type IN ?", opts.Types)
		}

		// first count all certificates
		err := query.Model(&models.Certificate{}).Count(&totalRows).Error
		if err != nil {
			return fmt.Errorf("error during counting certificates: %w", err)
		}

		// we need to apply scopes here again because count is being affected by offset otherwise
		query.Scopes(
			scopes.UserID(userID),
			scopes.Paginate(page),
			scopes.Preload("CertificateFields"),
		)

		// then find certificates with applied filters
		err = query.Find(&certificates).Error
		if err != nil {
			return fmt.Errorf("error during finding certificates: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, -1, fmt.Errorf("failed to list certificates: %w", err)
	}

	return certificates, totalRows, nil
}

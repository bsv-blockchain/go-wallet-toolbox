package service

import (
	"context"
	"log/slog"

	"github.com/bsv-blockchain/certifier-server-example/internal/domain"
	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/wallet"
)

type CertificateService struct {
	wallet    certificates.CertifierWallet
	validator domain.CertificateValidator
	logger    *slog.Logger
}

func NewCertificateService(wallet certificates.CertifierWallet, logger *slog.Logger) *CertificateService {
	return &CertificateService{
		wallet:    wallet,
		validator: domain.NewCertificateValidator(),
		logger:    logger,
	}
}

func (cs *CertificateService) SignCertificate(masterCertificate *certificates.MasterCertificate, counterPartyPubKey *ec.PublicKey) ([]byte, error) {
	ctx := context.Background()

	if err := cs.validator.ValidateRequest(masterCertificate); err != nil {
		return nil, err
	}

	counterparty := wallet.Counterparty{
		Type:         wallet.CounterpartyTypeOther,
		Counterparty: counterPartyPubKey,
	}

	fields, err := certificates.DecryptFields(
		ctx,
		cs.wallet,
		masterCertificate.MasterKeyring,
		masterCertificate.Fields,
		counterparty,
		false,
		"",
	)
	if err != nil {
		return nil, err
	}

	if err = cs.validator.ValidateDecryptedFields(fields); err != nil {
		return nil, err
	}

	stringFields := domain.ConvertFieldsToString(fields)

	signedMasterCert, err := certificates.IssueCertificateForSubject(
		ctx,
		cs.wallet,
		counterparty,
		stringFields,
		string(masterCertificate.Type),
		nil,
		"",
	)
	if err != nil {
		return nil, err
	}

	return signedMasterCert.ToBinary(true)
}

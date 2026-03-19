package domain

import (
	"context"
	"errors"

	"github.com/bsv-blockchain/certifier-server-example/internal/constants"
	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	"github.com/bsv-blockchain/go-sdk/wallet"
)

type CertificateService interface {
	SignCertificate(ctx context.Context, masterCert *certificates.MasterCertificate, counterparty wallet.Counterparty) ([]byte, error)
}

type CertificateValidator interface {
	ValidateRequest(masterCert *certificates.MasterCertificate) error
	ValidateDecryptedFields(fields map[wallet.CertificateFieldNameUnder50Bytes]string) error
}

type certificateValidator struct{}

func NewCertificateValidator() CertificateValidator {
	return &certificateValidator{}
}

func (v *certificateValidator) ValidateRequest(masterCert *certificates.MasterCertificate) error {
	if masterCert == nil {
		return certificates.ErrInvalidMasterCertificate
	}

	if len(masterCert.Type) == 0 {
		return errors.New("empty certificate type")
	}

	if len(masterCert.Fields) == 0 {
		return errors.New("empty certificate subject")
	}

	if len(masterCert.MasterKeyring) == 0 {
		return certificates.ErrMissingMasterKeyring
	}

	if masterCert.Type != constants.SupportedCertType {
		return errors.New("unsupported certificate type")
	}

	return nil
}

func (v *certificateValidator) ValidateDecryptedFields(fields map[wallet.CertificateFieldNameUnder50Bytes]string) error {
	var err error
	if fields[constants.EmailField] == "" {
		err = errors.Join(err, errors.New("email field not decrypted"))
	}
	if fields[constants.FirstNameField] == "" {
		err = errors.Join(err, errors.New("firstName field not decrypted"))
	}
	if fields[constants.LastNameField] == "" {
		err = errors.Join(err, errors.New("lastName field not decrypted"))
	}
	return err
}

func ConvertFieldsToString(fields map[wallet.CertificateFieldNameUnder50Bytes]string) map[string]string {
	stringFields := make(map[string]string)
	for k, v := range fields {
		stringFields[string(k)] = v
	}
	return stringFields
}

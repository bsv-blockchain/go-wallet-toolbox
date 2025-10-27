package storage

import (
	"encoding/base64"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func tableCertificateXFieldsToModelFields(userID int) func(*wdk.TableCertificateField) *models.CertificateField {
	return func(t *wdk.TableCertificateField) *models.CertificateField {
		return &models.CertificateField{
			FieldName:  t.FieldName,
			FieldValue: t.FieldValue,
			MasterKey:  string(t.MasterKey),
			UserID:     userID,
		}
	}
}

func certModelToResult(model *entity.Certificate) (*wdk.CertificateResult, error) {
	keyring, err := certificateModelFieldsToKeyringResult(model.CertificateFields)
	if err != nil {
		return nil, fmt.Errorf("failed to convert certificate model fields to keyring: %w", err)
	}

	return &wdk.CertificateResult{
		Verifier: wdk.VerifierString(model.Verifier),
		Keyring:  keyring,
		WalletCertificate: wdk.WalletCertificate{
			Type:               primitives.Base64String(model.Type),
			Subject:            primitives.PubKeyHex(model.Subject),
			SerialNumber:       primitives.Base64String(model.SerialNumber),
			Certifier:          primitives.PubKeyHex(model.Certifier),
			RevocationOutpoint: primitives.OutpointString(model.RevocationOutpoint),
			Signature:          primitives.HexString(model.Signature),
			Fields:             certificateModelFieldsToFieldsResult(model.CertificateFields),
		},
	}, nil
}

func certificateModelFieldsToKeyringResult(fields []entity.CertificateField) (wdk.KeyringMap, error) {
	result := make(wdk.KeyringMap)
	for _, field := range fields {
		val := field.FieldValue
		_, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			return nil, fmt.Errorf("failed to decode string %s, expected valid Base64 string: %w", val, err)
		}

		result[primitives.StringUnder50Bytes(field.FieldName)] = primitives.Base64String(val)
	}

	return result, nil
}

func certificateModelFieldsToFieldsResult(fields []entity.CertificateField) map[primitives.StringUnder50Bytes]string {
	result := make(map[primitives.StringUnder50Bytes]string, len(fields))
	for _, field := range fields {
		result[primitives.StringUnder50Bytes(field.FieldName)] = field.FieldValue
	}

	return result
}

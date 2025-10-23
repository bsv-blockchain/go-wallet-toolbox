package storage

import (
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

func certModelToResult(model *entity.Certificate) *wdk.CertificateResult {
	return &wdk.CertificateResult{
		Verifier: wdk.VerifierString(model.Verifier),
		Keyring:  certificateModelFieldsToKeyringResult(model.CertificateFields),
		WalletCertificate: wdk.WalletCertificate{
			Type:               primitives.Base64String(model.Type),
			Subject:            primitives.PubKeyHex(model.Subject),
			SerialNumber:       primitives.Base64String(model.SerialNumber),
			Certifier:          primitives.PubKeyHex(model.Certifier),
			RevocationOutpoint: primitives.OutpointString(model.RevocationOutpoint),
			Signature:          primitives.HexString(model.Signature),
			Fields:             certificateModelFieldsToFieldsResult(model.CertificateFields),
		},
	}
}

func certificateModelFieldsToKeyringResult(fields []entity.CertificateField) wdk.KeyringMap {
	result := make(wdk.KeyringMap, len(fields))
	for _, field := range fields {
		result[primitives.StringUnder50Bytes(field.FieldName)] = primitives.Base64String(field.FieldValue)
	}

	return result
}

func certificateModelFieldsToFieldsResult(fields []entity.CertificateField) map[primitives.StringUnder50Bytes]string {
	result := make(map[primitives.StringUnder50Bytes]string, len(fields))
	for _, field := range fields {
		result[primitives.StringUnder50Bytes(field.FieldName)] = field.FieldValue
	}

	return result
}

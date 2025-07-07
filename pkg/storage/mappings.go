package storage

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
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

func listCertificatesArgsToActionParams(args wdk.ListCertificatesArgs) repo.ListCertificatesActionParams {
	opts := repo.ListCertificatesActionParams{
		Limit:  args.Limit,
		Offset: args.Offset,
	}
	types := args.Types
	certifiers := args.Certifiers

	if args.Partial != nil {
		opts.SerialNumber = args.Partial.SerialNumber
		opts.Subject = args.Partial.Subject
		opts.RevocationOutpoint = args.Partial.RevocationOutpoint
		opts.Signature = args.Partial.Signature

		if args.Partial.Type != nil {
			types = append(types, *args.Partial.Type)
		}

		if args.Partial.Certifier != nil {
			certifiers = append(certifiers, *args.Partial.Certifier)
		}
	}

	opts.Types = types
	opts.Certifiers = certifiers

	return opts
}

func certModelToResult(model *models.Certificate) *wdk.CertificateResult {
	return &wdk.CertificateResult{
		Verifier: model.Verifier,
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

func certificateModelFieldsToKeyringResult(fields []*models.CertificateField) map[primitives.StringUnder50Bytes]primitives.Base64String {
	result := make(map[primitives.StringUnder50Bytes]primitives.Base64String, len(fields))
	for _, field := range fields {
		result[primitives.StringUnder50Bytes(field.FieldName)] = primitives.Base64String(field.FieldValue)
	}

	return result
}

func certificateModelFieldsToFieldsResult(fields []*models.CertificateField) map[primitives.StringUnder50Bytes]string {
	result := make(map[primitives.StringUnder50Bytes]string, len(fields))
	for _, field := range fields {
		result[primitives.StringUnder50Bytes(field.FieldName)] = field.FieldValue
	}

	return result
}

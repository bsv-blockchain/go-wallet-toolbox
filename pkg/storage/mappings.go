package storage

import (
	"fmt"
	"math"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
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

func listOutputsArgsToFilterParams(args wdk.ListOutputsArgs) entity.ListOutputsFilter {

	limit := must.ConvertToIntFromUnsigned(to.NoMoreThan(args.Limit, 10000))
	offset := must.ConvertToIntFromUnsigned(to.NoMoreThan(args.Offset, math.MaxInt))

	return entity.ListOutputsFilter{
		Basket:                    string(args.Basket),
		Tags:                      slices.Map(args.Tags, func(t primitives.StringUnder300) string { return string(t) }),
		TagQueryMode:              args.TagQueryMode,
		IncludeLockingScripts:     args.IncludeLockingScripts,
		IncludeTransactions:       args.IncludeTransactions,
		IncludeCustomInstructions: args.IncludeCustomInstructions,
		IncludeTags:               args.IncludeTags,
		IncludeLabels:             args.IncludeLabels,
		Limit:                     limit,
		Offset:                    offset,
		KnownTxids:                args.KnownTxids,
	}
}

func outputModelToResult(m *wdk.TableOutput) *wdk.WalletOutput {
	result := &wdk.WalletOutput{
		Satoshis:           primitives.SatoshiValue(must.ConvertToUInt64(m.Satoshis)),
		Spendable:          m.Spendable,
		CustomInstructions: m.CustomInstructions,
	}

	if m.TxID != nil {
		outpoint := fmt.Sprintf("%s.%d", *m.TxID, m.Vout)
		result.Outpoint = primitives.OutpointString(outpoint)
	}

	if m.LockingScript != nil {
		result.LockingScript = to.Ptr(primitives.HexString(*m.LockingScript))
	}

	return result
}

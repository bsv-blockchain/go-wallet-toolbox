package mapping

import (
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func MapToCertificateFieldNameUnder50BytesSlice(fields map[sdk.CertificateFieldNameUnder50Bytes]sdk.StringBase64) ([]sdk.CertificateFieldNameUnder50Bytes, error) {
	const (
		minLength = 1
		maxLength = 50
	)

	out := make([]sdk.CertificateFieldNameUnder50Bytes, 0, len(fields))
	for name := range fields {
		if len(name) < minLength || len(name) > maxLength {
			return nil, fmt.Errorf("invalid field name %q: must be between 1 and 50 bytes", name)
		}
		out = append(out, name)
	}

	return out, nil
}

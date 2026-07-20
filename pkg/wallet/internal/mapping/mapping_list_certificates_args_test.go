package mapping_test

import (
	"encoding/base64"
	"testing"

	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestMapListCertificatesArgs_ShortCertificateTypeNoZeroPad(t *testing.T) {
	t.Parallel()

	// given: TS-style short type ("CommonSource identity") zero-padded into CertificateType
	const shortB64 = "Q29tbW9uU291cmNlIGlkZW50aXR5"
	raw, err := base64.StdEncoding.DecodeString(shortB64)
	require.NoError(t, err)

	var certType wallet.CertificateType
	copy(certType[:], raw)

	// naive full-array encode produces a different string (the bug)
	paddedWire := base64.StdEncoding.EncodeToString(certType[:])
	require.NotEqual(t, shortB64, paddedWire)

	// when:
	_, types := mapping.MapListCertificatesArgs(wallet.ListCertificatesArgs{
		Types: []wallet.CertificateType{certType},
	})

	// then: mapped type matches original short base64 (no zero-pad re-encode corruption)
	require.Equal(t, []primitives.Base64String{shortB64}, types)
}

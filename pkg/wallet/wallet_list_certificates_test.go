package wallet_test

import (
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/stretchr/testify/require"
)

func (s *WalletTestSuite) TestListCertificates() {
	t := s.T()

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	aliceWallet := given.AliceWalletWithStorage(s.StorageType)

	// and:
	_, err := aliceWallet.AcquireCertificate(t.Context(), wallet.AcquireCertificateArgs{}, fixtures.DefaultOriginator)
	require.NoError(t, err)

	// when:
	args := wallet.ListCertificatesArgs{
		Types: []wallet.CertificateType{},
	}

	res, err := aliceWallet.ListCertificates(t.Context(), args, fixtures.DefaultOriginator)

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
}

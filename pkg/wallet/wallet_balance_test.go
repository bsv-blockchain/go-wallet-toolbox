package wallet_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func (s *WalletTestSuite) TestWalletBalance_EmptyWallet() {
	t := s.T()

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	aliceWallet := given.AliceWalletWithStorage(s.StorageType)

	// when:
	balance, err := aliceWallet.Balance(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, uint64(0), balance)
}

func (s *WalletTestSuite) TestWalletBalance_AfterTopUp() {
	t := s.T()

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	aliceWallet := given.AliceWalletWithStorage(s.StorageType)

	const topUpSatoshis = uint64(5000)
	given.Faucet(aliceWallet).TopUp(satoshi.MustFrom(topUpSatoshis))

	// when:
	balance, err := aliceWallet.Balance(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, topUpSatoshis, balance)
}

func (s *WalletTestSuite) TestWalletBalance_MultipleTopUps() {
	t := s.T()

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	aliceWallet := given.AliceWalletWithStorage(s.StorageType)

	faucet := given.Faucet(aliceWallet)
	faucet.TopUp(satoshi.MustFrom(1000))
	faucet.TopUp(satoshi.MustFrom(2000))
	faucet.TopUp(satoshi.MustFrom(300))

	// when:
	balance, err := aliceWallet.Balance(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, uint64(3300), balance)
}

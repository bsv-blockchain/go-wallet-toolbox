package testabilities

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
)

type BitailsServiceFixture interface {
	testservices.ServicesFixture
	NewBitailsService() *bitails.Bitails
}

type bitailsServiceFixture struct {
	testservices.ServicesFixture

	t testing.TB
}

func Given(t testing.TB) BitailsServiceFixture {
	return &bitailsServiceFixture{
		ServicesFixture: testservices.GivenServices(t),
		t:               t,
	}
}

func (f *bitailsServiceFixture) NewBitailsService() *bitails.Bitails {
	logger := logging.NewTestLogger(f.t)
	httpClient := f.Bitails().HttpClient()
	network := f.Network()

	config := to.OptionsWithDefault(defs.Bitails{
		APIKey: "",
	})

	return bitails.New(httpClient, logger, network, config)
}

func (f *bitailsServiceFixture) HashFromHex(hexStr string) *chainhash.Hash {
	return HashFromHex(f.t, hexStr)
}

func (f *bitailsServiceFixture) FakeHeaderHexWithMerkleRoot(root string) string {
	return FakeHeaderHexWithMerkleRoot(f.t, root)
}

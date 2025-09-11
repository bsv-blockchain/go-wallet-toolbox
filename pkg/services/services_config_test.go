package services_test

import (
	"context"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var mockTxID = testvectors.GivenTX().WithInput(10).WithP2PKHOutput(9).ID().String()

func TestServicesConfig_SingleMethodCustomService(t *testing.T) {
	given := testservices.GivenServices(t)

	// and:
	expectedRawTx := []byte{0, 1, 2}
	singleMethodCustomService := services.Implementation{
		RawTx: func(ctx context.Context, txID string) (*wdk.RawTxResult, error) {
			return &wdk.RawTxResult{TxID: txID, RawTx: expectedRawTx}, nil
		},
	}

	// and:
	service := given.Services().
		Opts(services.WithCustomImplementation("custom", singleMethodCustomService)).
		New()

	// when:
	result, err := service.RawTx(t.Context(), mockTxID)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, expectedRawTx, result.RawTx)
}

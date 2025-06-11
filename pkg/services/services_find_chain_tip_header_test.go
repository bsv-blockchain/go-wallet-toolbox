package services_test

import (
	"net/http"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/dto"
	"github.com/stretchr/testify/require"
)

// TODO: Add generic type responses to the wdk pkg.
func TestFindChainTipHeader(t *testing.T) {
	// given:
	expectedBlockResponse := dto.BlockResponse{
		Hash:              "00000000000000000c5f0e00dadaf092df83f98d4dd7b5c271d4ea77840d9616",
		Confirmations:     1,
		Size:              15931220,
		Height:            900769,
		Version:           603979776,
		VersionHex:        "24000000",
		MerkleRoot:        "d2c956bb4e4630d5e3f7d3d2188033708010b7e39fe437688a885d28617df3e9",
		Time:              1749640390,
		Mediantime:        1749638570,
		Nonce:             3490285233,
		Bits:              "1811a0fe",
		Difficulty:        62368971637.7024,
		Chainwork:         "0000000000000000000000000000000000000000016674eb118c112eb06e2669",
		PreviousBlockHash: "00000000000000001018b9d31586ff13e4797fa527d1bc74d33424853940c18c",
		NextBlockHash:     "",
		NTx:               2928,
		NumTx:             2928,
	}

	expectedBlock, err := whatsonchain.ConvertToBlockHeader(expectedBlockResponse)
	require.NoError(t, err)

	given := testservices.GivenServices(t)
	given.WhatsOnChain().WillRespondWithTipBlockHeader(http.StatusOK, nil, expectedBlockResponse)
	service := given.Services().WithDefaultConfig()

	// when:
	actualBlock, err := service.FindChainTipHeader(t.Context())

	// then:
	require.NoError(t, err)
	require.Equal(t, expectedBlock, actualBlock)
}

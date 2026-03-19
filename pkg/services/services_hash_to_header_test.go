package services_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	btb "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
)

func TestWalletServices_HashToHeader_SuccessCases(t *testing.T) {
	blockHash := btb.TestHex
	blockHeight := testabilities.TestBlockHeight
	version := 536870912
	merkleRoot := testabilities.TestMerkleRootHex
	time := uint32(1712345678)
	nonce := 123456789
	bits := "1803a30c"
	prevHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	tests := []struct {
		name  string
		setup func(testservices.ServicesFixture)
	}{
		{
			name: "WhatsOnChain returns valid header",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().WillRespondWithBlockHeader(http.StatusOK, blockHash,
					testabilities.ValidBlockHeaderJSON(blockHash, blockHeight, version, merkleRoot, time, nonce, bits, prevHash))
			},
		},
		{
			name: "Bitails returns valid header",
			setup: func(f testservices.ServicesFixture) {
				rawHeader := btb.ValidBlockHeaderRaw()

				err := f.WhatsOnChain().WillBeUnreachable()
				require.Error(t, err)

				f.Bitails().WillReturnBlockHeader(blockHash, rawHeader)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fix := testservices.GivenServices(t)
			tc.setup(fix)

			svc := fix.Services().Config(testservices.WithEnabledBitails(true)).New()

			// when:
			header, err := svc.HashToHeader(t.Context(), blockHash)

			// then:
			require.NoError(t, err)
			require.NotNil(t, header)
			require.Equal(t, blockHash, header.Hash)
		})
	}
}

func TestWalletServices_HashToHeader_ErrorCases(t *testing.T) {
	tests := []struct {
		name  string
		setup func(testservices.ServicesFixture)
	}{
		{
			name: "WhatsOnChain unreachable",
			setup: func(f testservices.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
			},
		},
		{
			name: "WhatsOnChain returns malformed JSON",
			setup: func(f testservices.ServicesFixture) {
				blockHash := testabilities.TestTargetHash
				f.WhatsOnChain().WillReturnMalformedBlockHeader(blockHash)
			},
		},
		{
			name: "WhatsOnChain returns invalid header hex",
			setup: func(f testservices.ServicesFixture) {
				blockHash := testabilities.TestTargetHash
				f.WhatsOnChain().WillRespondWithBlockHeader(http.StatusOK, blockHash, "not-a-hex")
			},
		},
		{
			name: "WhatsOnChain returns incomplete header",
			setup: func(f testservices.ServicesFixture) {
				blockHash := testabilities.TestTargetHash
				f.WhatsOnChain().WillRespondWithBlockHeader(http.StatusOK, blockHash, testabilities.IncompleteBlockHeaderRaw())
			},
		},
		{
			name: "Bitails returns invalid hex",
			setup: func(f testservices.ServicesFixture) {
				blockHash := testabilities.TestTargetHash
				f.Bitails().WillReturnBlockHeader(blockHash, "badhex")
			},
		},
		{
			name: "Bitails returns too short header",
			setup: func(f testservices.ServicesFixture) {
				blockHash := testabilities.TestTargetHash
				f.Bitails().WillReturnBlockHeader(blockHash, "00")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fix := testservices.GivenServices(t)
			tc.setup(fix)

			svc := fix.Services().Config(testservices.WithEnabledBitails(true)).New()

			// when:
			res, err := svc.HashToHeader(t.Context(), testabilities.TestTargetHash)

			// then:
			require.Error(t, err)
			require.Nil(t, res)
		})
	}
}

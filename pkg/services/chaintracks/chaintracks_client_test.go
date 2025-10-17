package chaintracks_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func TestChaintracksClient_Init(t *testing.T) {
	t.Run("should panic on empty baseURL", func(t *testing.T) {
		require.Panics(t, func() {
			chaintracks.NewClient(logging.NewTestLogger(t), defs.NetworkMainnet, "")
		})
	})

	t.Run("should panic on invalid chain", func(t *testing.T) {
		require.Panics(t, func() {
			chaintracks.NewClient(logging.NewTestLogger(t), "invalid-chain", "http://valid-url.com")
		})
	})
}

func TestChaintracksClient_GetInfo(t *testing.T) {
	tests := map[string]struct {
		ResponseCode int
		ResponseBody any
		Then         func(t *testing.T, info *chaintracks.InfoResponse, err error)
	}{
		"should return correct chain": {
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[chaintracks.InfoResponse]{
				Status: "success",
				Value:  &chaintracks.InfoResponse{Chain: defs.NetworkMainnet},
			},
			Then: func(t *testing.T, info *chaintracks.InfoResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, info)
				require.Equal(t, defs.NetworkMainnet, info.Chain)
			},
		},
		"should return error on non-200 response": {
			ResponseCode: 500,
			ResponseBody: `{"status":"error","message":"internal server error"}`,
			Then: func(t *testing.T, info *chaintracks.InfoResponse, err error) {
				require.Error(t, err)
				require.Nil(t, info)
			},
		},
		"should return error on invalid JSON": {
			ResponseCode: 200,
			ResponseBody: `{"status":"success","value":{invalid json}}`,
			Then: func(t *testing.T, info *chaintracks.InfoResponse, err error) {
				require.Error(t, err)
				require.Nil(t, info)
			},
		},
		"should return error on error status in response": {
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[chaintracks.InfoResponse]{
				Status: "error",
				Value:  nil,
			},
			Then: func(t *testing.T, info *chaintracks.InfoResponse, err error) {
				require.Error(t, err)
				require.Nil(t, info)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)

			mockServer := given.MockServer()
			mockServer.
				WillRespondOn("/getInfo", "GET").
				WithJSONResponse(test.ResponseCode, test.ResponseBody)

			chaintr := chaintracks.NewClient(logging.NewTestLogger(t), defs.NetworkMainnet, "http://mock-chaintracks.com", chaintracks.WithRestyClient(mockServer.HttpClient()))

			// when:
			info, err := chaintr.GetInfo(t.Context())

			// then:
			test.Then(t, info, err)
		})
	}
}

func TestChaintracksClient_GetPresentHeight(t *testing.T) {
	tests := map[string]struct {
		ResponseCode int
		ResponseBody any
		Then         func(t *testing.T, height uint32, err error)
	}{
		"should return correct chain": {
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[uint32]{
				Status: "success",
				Value:  to.Ptr[uint32](1000),
			},
			Then: func(t *testing.T, height uint32, err error) {
				require.NoError(t, err)
				require.Equal(t, uint32(1000), height)
			},
		},
		"should return error on non-200 response": {
			ResponseCode: 500,
			ResponseBody: `{"status":"error","message":"internal server error"}`,
			Then: func(t *testing.T, height uint32, err error) {
				require.Error(t, err)
				require.Equal(t, uint32(0), height)
			},
		},
		"should return error on invalid JSON": {
			ResponseCode: 200,
			ResponseBody: `{"status":"success","value":{invalid json}}`,
			Then: func(t *testing.T, height uint32, err error) {
				require.Error(t, err)
				require.Equal(t, uint32(0), height)
			},
		},
		"should return error on error status in response": {
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[uint]{
				Status: "error",
				Value:  nil,
			},
			Then: func(t *testing.T, height uint32, err error) {
				require.Error(t, err)
				require.Equal(t, uint32(0), height)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)

			mockServer := given.MockServer()
			mockServer.
				WillRespondOn("/getPresentHeight", "GET").
				WithJSONResponse(test.ResponseCode, test.ResponseBody)

			chaintr := chaintracks.NewClient(logging.NewTestLogger(t), defs.NetworkMainnet, "http://mock-chaintracks.com", chaintracks.WithRestyClient(mockServer.HttpClient()))

			// when:
			height, err := chaintr.GetPresentHeight(t.Context())

			// then:
			test.Then(t, height, err)
		})
	}
}

func TestChaintracksClient_FindChainTipHashHex(t *testing.T) {
	tests := map[string]struct {
		ResponseCode int
		ResponseBody any
		Then         func(t *testing.T, hash string, err error)
	}{
		"should return correct hash": {
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[string]{
				Status: "success",
				Value:  to.Ptr("0000000000000000000a7b3c4d5e6f708090a0b0c0d0e0f1011121314151617"),
			},
			Then: func(t *testing.T, hash string, err error) {
				require.NoError(t, err)
				require.Equal(t, "0000000000000000000a7b3c4d5e6f708090a0b0c0d0e0f1011121314151617", hash)
			},
		},
		"should return error on non-200 response": {
			ResponseCode: 500,
			ResponseBody: `{"status":"error","message":"internal server error"}`,
			Then: func(t *testing.T, hash string, err error) {
				require.Error(t, err)
				require.Empty(t, hash)
			},
		},
		"should return error on invalid JSON": {
			ResponseCode: 200,
			ResponseBody: `{"status":"success","value":{invalid json}}`,
			Then: func(t *testing.T, hash string, err error) {
				require.Error(t, err)
				require.Empty(t, hash)
			},
		},
		"should return error on error status in response": {
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[string]{
				Status: "error",
				Value:  nil,
			},
			Then: func(t *testing.T, hash string, err error) {
				require.Error(t, err)
				require.Empty(t, hash)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)

			mockServer := given.MockServer()
			mockServer.
				WillRespondOn("/findChainTipHashHex", "GET").
				WithJSONResponse(test.ResponseCode, test.ResponseBody)

			chaintr := chaintracks.NewClient(logging.NewTestLogger(t), defs.NetworkMainnet, "http://mock-chaintracks.com", chaintracks.WithRestyClient(mockServer.HttpClient()))

			// when:
			hash, err := chaintr.FindChainTipHashHex(t.Context())

			// then:
			test.Then(t, hash, err)
		})
	}
}

func TestChaintracksClient_FindChainTipHeader(t *testing.T) {
	tests := map[string]struct {
		ResponseCode int
		ResponseBody any
		Then         func(t *testing.T, header *chaintracks.BlockHeader, err error)
	}{
		"should return correct header": {
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[chaintracks.BlockHeader]{
				Status: "success",
				Value:  correctBlockHeader,
			},
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.NoError(t, err)
				require.Equal(t, correctBlockHeader, header)
			},
		},
		"should return error on non-200 response": {
			ResponseCode: 500,
			ResponseBody: `{"status":"error","message":"internal server error"}`,
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.Error(t, err)
				require.Nil(t, header)
			},
		},
		"should return error on invalid JSON": {
			ResponseCode: 200,
			ResponseBody: `{"status":"success","value":{invalid json}}`,
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.Error(t, err)
				require.Nil(t, header)
			},
		},
		"should return error on error status in response": {
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[chaintracks.BlockHeader]{
				Status: "error",
				Value:  nil,
			},
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.Error(t, err)
				require.Nil(t, header)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)

			mockServer := given.MockServer()
			mockServer.
				WillRespondOn("/findChainTipHeaderHex", "GET").
				WithJSONResponse(test.ResponseCode, test.ResponseBody)

			chaintr := chaintracks.NewClient(logging.NewTestLogger(t), defs.NetworkMainnet, "http://mock-chaintracks.com", chaintracks.WithRestyClient(mockServer.HttpClient()))

			// when:
			blockHeader, err := chaintr.FindChainTipHeaderHex(t.Context())

			// then:
			test.Then(t, blockHeader, err)
		})
	}
}

func TestChaintracksClient_FindHeaderHexForHeight(t *testing.T) {
	tests := map[string]struct {
		Height       uint32
		ResponseCode int
		ResponseBody any
		Then         func(t *testing.T, header *chaintracks.BlockHeader, err error)
	}{
		"should return correct header": {
			Height:       918934,
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[chaintracks.BlockHeader]{
				Status: "success",
				Value:  correctBlockHeader,
			},
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.NoError(t, err)
				require.Equal(t, correctBlockHeader, header)
			},
		},
		"should handle not-found as success with no value": {
			Height:       123456,
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[chaintracks.BlockHeader]{
				Status: "success",
				Value:  nil,
			},
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.ErrorIs(t, err, wdk.ErrNotFoundError)
				require.Nil(t, header)
			},
		},
		"should return error on non-200 response": {
			Height:       918934,
			ResponseCode: 500,
			ResponseBody: `{"status":"error","message":"internal server error"}`,
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.Error(t, err)
				require.Nil(t, header)
			},
		},
		"should return error on invalid JSON": {
			Height:       918934,
			ResponseCode: 200,
			ResponseBody: `{"status":"success","value":{invalid json}}`,
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.Error(t, err)
				require.Nil(t, header)
			},
		},
		"should return error on error status in response": {
			Height:       918934,
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[chaintracks.BlockHeader]{
				Status: "error",
				Value:  nil,
			},
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.Error(t, err)
				require.Nil(t, header)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)

			mockServer := given.MockServer()
			mockServer.
				WillRespondOn("/findHeaderHexForHeight", "GET").
				WithJSONResponse(test.ResponseCode, test.ResponseBody)

			chaintr := chaintracks.NewClient(logging.NewTestLogger(t), defs.NetworkMainnet, "http://mock-chaintracks.com", chaintracks.WithRestyClient(mockServer.HttpClient()))

			// when:
			blockHeader, err := chaintr.FindHeaderHexForHeight(t.Context(), test.Height)

			// then:
			test.Then(t, blockHeader, err)
		})
	}
}

func TestChaintracksClient_FindHeaderHexForBlockHash(t *testing.T) {
	tests := map[string]struct {
		BlockHash    string
		ResponseCode int
		ResponseBody any
		Then         func(t *testing.T, header *chaintracks.BlockHeader, err error)
	}{
		"should return correct header": {
			BlockHash:    correctBlockHeader.Hash,
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[chaintracks.BlockHeader]{
				Status: "success",
				Value:  correctBlockHeader,
			},
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.NoError(t, err)
				require.Equal(t, correctBlockHeader, header)
			},
		},
		"should handle not-found as success with no value": {
			BlockHash:    correctBlockHeader.Hash,
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[chaintracks.BlockHeader]{
				Status: "success",
				Value:  nil,
			},
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.ErrorIs(t, err, wdk.ErrNotFoundError)
				require.Nil(t, header)
			},
		},
		"should return error on non-200 response": {
			BlockHash:    correctBlockHeader.Hash,
			ResponseCode: 500,
			ResponseBody: `{"status":"error","message":"internal server error"}`,
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.Error(t, err)
				require.Nil(t, header)
			},
		},
		"should return error on invalid JSON": {
			BlockHash:    correctBlockHeader.Hash,
			ResponseCode: 200,
			ResponseBody: `{"status":"success","value":{invalid json}}`,
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.Error(t, err)
				require.Nil(t, header)
			},
		},
		"should return error on error status in response": {
			BlockHash:    correctBlockHeader.Hash,
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[chaintracks.BlockHeader]{
				Status: "error",
				Value:  nil,
			},
			Then: func(t *testing.T, header *chaintracks.BlockHeader, err error) {
				require.Error(t, err)
				require.Nil(t, header)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)

			mockServer := given.MockServer()
			mockServer.
				WillRespondOn("/findHeaderHexForBlockHash", "GET").
				WithJSONResponse(test.ResponseCode, test.ResponseBody)

			chaintr := chaintracks.NewClient(logging.NewTestLogger(t), defs.NetworkMainnet, "http://mock-chaintracks.com", chaintracks.WithRestyClient(mockServer.HttpClient()))

			// when:
			blockHeader, err := chaintr.FindHeaderHexForBlockHash(t.Context(), test.BlockHash)

			// then:
			test.Then(t, blockHeader, err)
		})
	}
}

var correctBlockHeader = &chaintracks.BlockHeader{
	Height: 918934,
	Hash:   "00000000000000000165924d2b7e41fd586d88e02f846ea6428d37c51f97db31",
	BaseBlockHeader: chaintracks.BaseBlockHeader{
		Version:          594616320,
		PreviousHash:     "000000000000000007dce4ee864f0569066bcded7a6a3eca9b977eebf9cb3926",
		MerkleRoot:       "f4afaab11ec4921a59c9eb1e49435aeaaef6e4396ee9d4a77ef88fdd60057928",
		Time:             1760620895,
		Bits:             405131341,
		Nonce:            2411550733,
		HeaderID:         2025,
		PreviousHeaderID: to.Ptr[uint](2024),
		Chainwork:        "0000000000000000000000000000000000000000016934b62084adb28c318e3c",
		IsChainTip:       true,
		IsActive:         true,
	},
}

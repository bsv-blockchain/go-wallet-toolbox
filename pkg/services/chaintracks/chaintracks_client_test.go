package chaintracks_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/internal/testabilities"
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
		Then         func(t *testing.T, height uint, err error)
	}{
		"should return correct chain": {
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[uint]{
				Status: "success",
				Value:  to.Ptr[uint](1000),
			},
			Then: func(t *testing.T, height uint, err error) {
				require.NoError(t, err)
				require.Equal(t, uint(1000), height)
			},
		},
		"should return error on non-200 response": {
			ResponseCode: 500,
			ResponseBody: `{"status":"error","message":"internal server error"}`,
			Then: func(t *testing.T, height uint, err error) {
				require.Error(t, err)
				require.Equal(t, uint(0), height)
			},
		},
		"should return error on invalid JSON": {
			ResponseCode: 200,
			ResponseBody: `{"status":"success","value":{invalid json}}`,
			Then: func(t *testing.T, height uint, err error) {
				require.Error(t, err)
				require.Equal(t, uint(0), height)
			},
		},
		"should return error on error status in response": {
			ResponseCode: 200,
			ResponseBody: chaintracks.ResponseFrame[uint]{
				Status: "error",
				Value:  nil,
			},
			Then: func(t *testing.T, height uint, err error) {
				require.Error(t, err)
				require.Equal(t, uint(0), height)
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

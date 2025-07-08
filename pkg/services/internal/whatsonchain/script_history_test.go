package whatsonchain_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ScriptHistoryTestSuite provides integration tests for script history functionality
type ScriptHistoryTestSuite struct {
	suite.Suite
	woc        *whatsonchain.WhatsOnChain
	transport  *httpmock.MockTransport
	httpClient *resty.Client
}

func TestScriptHistoryTestSuite(t *testing.T) {
	suite.Run(t, new(ScriptHistoryTestSuite))
}

func (s *ScriptHistoryTestSuite) SetupTest() {
	s.transport = httpmock.NewMockTransport()
	s.httpClient = resty.New()
	s.httpClient.SetTransport(s.transport)

	config := defs.WhatsOnChain{
		BroadcastDelay:  0,
		BSVExchangeRate: defs.BSVExchangeRate{},
	}

	s.woc = whatsonchain.New(
		s.httpClient,
		slog.Default(),
		defs.NetworkMainnet,
		config,
	)
}

func (s *ScriptHistoryTestSuite) TearDownTest() {
	s.transport.Reset()
}

func (s *ScriptHistoryTestSuite) TestGetScriptHistory_TypicalP2PKHAddress() {
	// given
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	confirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{
			{
				TxID:   "1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef1234567890",
				Height: to.Ptr(800000),
			},
			{
				TxID:   "9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba",
				Height: to.Ptr(800001),
			},
		},
		Error: "",
	}

	unconfirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{
			{
				TxID:   "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				Height: nil,
			},
		},
		Error: "",
	}

	s.registerConfirmedHistoryMock(scriptHash, http.StatusOK, confirmedResponse)
	s.registerUnconfirmedHistoryMock(scriptHash, http.StatusOK, unconfirmedResponse)

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().NoError(err)
	s.Assert().Equal(whatsonchain.ServiceName, result.Name)
	s.Assert().Equal(scriptHash, result.ScriptHash)
	s.Assert().Len(result.History, 3)

	s.Assert().Equal("1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef1234567890", result.History[0].TxHash)
	s.Assert().Equal(800000, *result.History[0].Height)

	s.Assert().Equal("9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba", result.History[1].TxHash)
	s.Assert().Equal(800001, *result.History[1].Height)

	s.Assert().Equal("abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", result.History[2].TxHash)
	s.Assert().Nil(result.History[2].Height)
}

func (s *ScriptHistoryTestSuite) TestGetScriptHistory_WithPaginationOptions() {
	// given
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"
	opts := &wdk.GetConfirmedScriptHistoryOpts{
		Order:         to.Ptr(wdk.ScriptHistoryOrderDesc),
		Limit:         to.Ptr(100),
		Height:        to.Ptr(800000),
		NextPageToken: to.Ptr("next_page_token_123"),
	}

	confirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{
			{
				TxID:   "1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef1234567890",
				Height: to.Ptr(800000),
			},
		},
		Error: "",
	}

	unconfirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{},
		Error:  "",
	}

	expectedURL := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/main/script/%s/confirmed/history?height=800000&limit=100&order=desc&token=next_page_token_123", scriptHash)
	s.transport.RegisterResponder(
		http.MethodGet,
		expectedURL,
		httpmock.NewJsonResponderOrPanic(http.StatusOK, confirmedResponse),
	)

	s.registerUnconfirmedHistoryMock(scriptHash, http.StatusOK, unconfirmedResponse)

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, opts)

	// then
	s.Require().NoError(err)
	s.Assert().Equal(whatsonchain.ServiceName, result.Name)
	s.Assert().Len(result.History, 1)
}

func (s *ScriptHistoryTestSuite) TestGetScriptHistory_EmptyHistory() {
	// given
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	confirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{},
		Error:  "",
	}

	unconfirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{},
		Error:  "",
	}

	s.registerConfirmedHistoryMock(scriptHash, http.StatusOK, confirmedResponse)
	s.registerUnconfirmedHistoryMock(scriptHash, http.StatusOK, unconfirmedResponse)

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().NoError(err)
	s.Assert().Equal(whatsonchain.ServiceName, result.Name)
	s.Assert().Equal(scriptHash, result.ScriptHash)
	s.Assert().Empty(result.History)
}

func (s *ScriptHistoryTestSuite) TestGetScriptHistory_OnlyConfirmed() {
	// given
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	confirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{
			{
				TxID:   "1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef1234567890",
				Height: to.Ptr(800000),
			},
		},
		Error: "",
	}

	unconfirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{},
		Error:  "",
	}

	s.registerConfirmedHistoryMock(scriptHash, http.StatusOK, confirmedResponse)
	s.registerUnconfirmedHistoryMock(scriptHash, http.StatusOK, unconfirmedResponse)

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().NoError(err)
	s.Assert().Len(result.History, 1)
	s.Assert().NotNil(result.History[0].Height)
	s.Assert().Equal(800000, *result.History[0].Height)
}

func (s *ScriptHistoryTestSuite) TestGetScriptHistory_OnlyUnconfirmed() {
	// given
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	confirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{},
		Error:  "",
	}

	unconfirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{
			{
				TxID:   "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				Height: nil,
			},
		},
		Error: "",
	}

	s.registerConfirmedHistoryMock(scriptHash, http.StatusOK, confirmedResponse)
	s.registerUnconfirmedHistoryMock(scriptHash, http.StatusOK, unconfirmedResponse)

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().NoError(err)
	s.Assert().Len(result.History, 1)
	s.Assert().Nil(result.History[0].Height)
}

// Error scenarios
func (s *ScriptHistoryTestSuite) TestGetScriptHistory_ConfirmedAPIError() {
	// given
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	confirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{},
		Error:  "Internal server error",
	}

	s.registerConfirmedHistoryMock(scriptHash, http.StatusOK, confirmedResponse)

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "API error: Internal server error")
	s.Assert().Nil(result)
}

func (s *ScriptHistoryTestSuite) TestGetScriptHistory_UnconfirmedAPIError() {
	// given
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	confirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{
			{
				TxID:   "1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef1234567890",
				Height: to.Ptr(800000),
			},
		},
		Error: "",
	}

	unconfirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{},
		Error:  "Script not found",
	}

	s.registerConfirmedHistoryMock(scriptHash, http.StatusOK, confirmedResponse)
	s.registerUnconfirmedHistoryMock(scriptHash, http.StatusOK, unconfirmedResponse)

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "API error: Script not found")
	s.Assert().Nil(result)
}

func (s *ScriptHistoryTestSuite) TestGetScriptHistory_HTTPError() {
	// given
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	s.registerConfirmedHistoryMock(scriptHash, http.StatusNotFound, nil)

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "unexpected status code 404")
	s.Assert().Nil(result)
}

func (s *ScriptHistoryTestSuite) TestGetScriptHistory_ValidationErrors() {
	testCases := []struct {
		name          string
		scriptHash    string
		expectedError string
	}{
		{
			name:          "empty scripthash",
			scriptHash:    "",
			expectedError: "scripthash cannot be empty",
		},
		{
			name:          "too short scripthash",
			scriptHash:    "a914b7536c",
			expectedError: "invalid scripthash length: too short",
		},
		{
			name:          "too long scripthash",
			scriptHash:    "a914b7536c788d8ca2de4d867a2b5b02acef97f35aef488aca914b7536c788d8ca2de4d867a2b5b02acef97f35aef488ac",
			expectedError: "invalid scripthash length: too long",
		},
		{
			name:          "invalid hex characters",
			scriptHash:    "this is not valid base64!! this is not valid base64!!",
			expectedError: "invalid scripthash format",
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			result, err := s.woc.GetScriptHistory(context.Background(), tc.scriptHash, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
			assert.Nil(t, result)
		})
	}
}

func (s *ScriptHistoryTestSuite) TestGetScriptHistory_LargeHistory() {
	// given
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"
	var confirmedItems []wdk.ScriptHashHistoryItem
	for i := 0; i < 100; i++ {
		confirmedItems = append(confirmedItems, wdk.ScriptHashHistoryItem{
			TxID:   fmt.Sprintf("tx%096d", i),
			Height: to.Ptr(800000 + i),
		})
	}

	var unconfirmedItems []wdk.ScriptHashHistoryItem
	for i := 0; i < 10; i++ {
		unconfirmedItems = append(unconfirmedItems, wdk.ScriptHashHistoryItem{
			TxID:   fmt.Sprintf("unconfirmed_tx%086d", i),
			Height: nil,
		})
	}

	confirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: confirmedItems,
		Error:  "",
	}

	unconfirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: unconfirmedItems,
		Error:  "",
	}

	s.registerConfirmedHistoryMock(scriptHash, http.StatusOK, confirmedResponse)
	s.registerUnconfirmedHistoryMock(scriptHash, http.StatusOK, unconfirmedResponse)

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().NoError(err)
	s.Assert().Len(result.History, 110)

	for i := 0; i < 100; i++ {
		s.Assert().NotNil(result.History[i].Height)
		s.Assert().Equal(800000+i, *result.History[i].Height)
	}

	for i := 100; i < 110; i++ {
		s.Assert().Nil(result.History[i].Height)
	}
}

// Helper methods
func (s *ScriptHistoryTestSuite) registerConfirmedHistoryMock(scriptHash string, status int, response interface{}) {
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/main/script/%s/confirmed/history", scriptHash)
	if response != nil {
		s.transport.RegisterResponder(
			http.MethodGet,
			url,
			httpmock.NewJsonResponderOrPanic(status, response),
		)
	} else {
		s.transport.RegisterResponder(
			http.MethodGet,
			url,
			httpmock.NewStringResponder(status, ""),
		)
	}
}

func (s *ScriptHistoryTestSuite) registerUnconfirmedHistoryMock(scriptHash string, status int, response interface{}) {
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/main/script/%s/unconfirmed/history", scriptHash)
	if response != nil {
		s.transport.RegisterResponder(
			http.MethodGet,
			url,
			httpmock.NewJsonResponderOrPanic(status, response),
		)
	} else {
		s.transport.RegisterResponder(
			http.MethodGet,
			url,
			httpmock.NewStringResponder(status, ""),
		)
	}
}

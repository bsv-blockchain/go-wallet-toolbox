package services_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ScriptHistoryFixtureTestSuite demonstrates usage of the new fixture methods
type ScriptHistoryFixtureTestSuite struct {
	suite.Suite
	given testabilities.WoCServiceFixture
	woc   *whatsonchain.WhatsOnChain
}

func TestScriptHistoryFixtureTestSuite(t *testing.T) {
	suite.Run(t, new(ScriptHistoryFixtureTestSuite))
}

func (s *ScriptHistoryFixtureTestSuite) SetupTest() {
	s.given = testabilities.Given(s.T())
	s.woc = s.given.NewWoCService()
}

func (s *ScriptHistoryFixtureTestSuite) TearDownTest() {
	s.given.WhatsOnChain().Transport().Reset()
}

func (s *ScriptHistoryFixtureTestSuite) TestGetScriptHistory_UsingBuilderPattern() {
	// given
	scriptHashes := testservices.ValidScriptHashes()
	scriptHash := scriptHashes["script_hash_32"]

	s.given.WhatsOnChain().
		WithValidScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactions(5, 800000).
		WithUnconfirmedTransactions(2).
		SetupMocks(s.given.WhatsOnChain())

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().NoError(err)
	s.Assert().Equal(whatsonchain.ServiceName, result.Name)
	s.Assert().Equal(scriptHash, result.ScriptHash)
	s.Assert().Len(result.History, 7) // 5 confirmed + 2 unconfirmed

	for i := 0; i < 5; i++ {
		s.Assert().NotNil(result.History[i].Height)
		s.Assert().Equal(800000+i, *result.History[i].Height)
	}

	for i := 5; i < 7; i++ {
		s.Assert().Nil(result.History[i].Height)
	}
}

func (s *ScriptHistoryFixtureTestSuite) TestGetScriptHistory_UsingQueryFixture() {
	// given
	scriptHashes := testservices.ValidScriptHashes()
	scriptHash := scriptHashes["p2pkh_standard"]

	confirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{
			{
				TxID:   "tx_confirmed_001",
				Height: to.Ptr(800000),
			},
		},
		Error: "",
	}

	unconfirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{
			{
				TxID:   "tx_unconfirmed_001",
				Height: nil,
			},
		},
		Error: "",
	}

	s.given.WhatsOnChain().
		WhenQueryingScriptHistory(scriptHash).
		WillReturnConfirmedHistory(http.StatusOK, confirmedResponse)

	s.given.WhatsOnChain().
		WhenQueryingScriptHistory(scriptHash).
		WillReturnUnconfirmedHistory(http.StatusOK, unconfirmedResponse)

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().NoError(err)
	s.Assert().Len(result.History, 2)
	s.Assert().Equal("tx_confirmed_001", result.History[0].TxHash)
	s.Assert().Equal("tx_unconfirmed_001", result.History[1].TxHash)
}

func (s *ScriptHistoryFixtureTestSuite) TestGetConfirmedScriptHistory_WithPagination() {
	// given
	scriptHashes := testservices.ValidScriptHashes()
	scriptHash := scriptHashes["p2sh_standard"]

	opts := &wdk.GetConfirmedScriptHistoryOpts{
		Order:         to.Ptr(wdk.ScriptHistoryOrderDesc),
		Limit:         to.Ptr(50),
		Height:        to.Ptr(800000),
		NextPageToken: to.Ptr("pagination_token_123"),
	}

	response := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{
			{
				TxID:   "paginated_tx_001",
				Height: to.Ptr(800001),
			},
		},
		Error: "",
	}
	unconfirmedResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{},
		Error:  "",
	}

	s.given.WhatsOnChain().
		WhenQueryingScriptHistory(scriptHash).
		WillReturnConfirmedHistoryWithPagination(http.StatusOK, response, opts)

	s.given.WhatsOnChain().
		WhenQueryingScriptHistory(scriptHash).
		WillReturnUnconfirmedHistory(http.StatusOK, unconfirmedResponse)

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, opts)

	// then
	s.Require().NoError(err)
	s.Assert().Len(result.History, 1)
	s.Assert().Equal("paginated_tx_001", result.History[0].TxHash)
}

func (s *ScriptHistoryFixtureTestSuite) TestGetScriptHistory_ValidationErrors() {
	invalidHashes := testservices.InvalidScriptHashes()

	for name, testCase := range invalidHashes {
		s.T().Run(name, func(t *testing.T) {
			s.given.WhatsOnChain().WithScriptHistoryValidationError(testCase.Hash, testCase.ExpectedError)

			// when
			result, err := s.woc.GetScriptHistory(context.Background(), testCase.Hash, nil)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), testCase.ExpectedError)
			assert.Nil(t, result)
		})
	}
}

func (s *ScriptHistoryFixtureTestSuite) TestGetScriptHistory_EmptyHistory() {
	// given
	scriptHashes := testservices.ValidScriptHashes()
	scriptHash := scriptHashes["p2pkh_standard"]

	s.given.WhatsOnChain().
		WithValidScriptHistoryData().
		WithScriptHash(scriptHash).
		WithEmptyHistory().
		SetupMocks(s.given.WhatsOnChain())

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().NoError(err)
	s.Assert().Empty(result.History)
}

func (s *ScriptHistoryFixtureTestSuite) TestGetScriptHistory_LargeHistory() {
	// given
	scriptHashes := testservices.ValidScriptHashes()
	scriptHash := scriptHashes["script_hash_32"]

	s.given.WhatsOnChain().
		WithValidScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactions(1000, 700000).
		WithUnconfirmedTransactions(50).
		SetupMocks(s.given.WhatsOnChain())

	// when
	result, err := s.woc.GetScriptHistory(context.Background(), scriptHash, nil)

	// then
	s.Require().NoError(err)
	s.Assert().Len(result.History, 1050) 
	
	for i := 0; i < 1000; i++ {
		s.Assert().NotNil(result.History[i].Height)
	}
	for i := 1000; i < 1050; i++ {
		s.Assert().Nil(result.History[i].Height)
	}
}

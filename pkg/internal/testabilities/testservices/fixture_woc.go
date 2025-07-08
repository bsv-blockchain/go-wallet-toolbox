package testservices

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
)

type WhatsOnChainFixture interface {
	WillRespondWithRates(status int, content string, err error)
	WillRespondWithRawTx(status int, txID, rawTx string, err error)
	OnTipBlockHeaderWillRespondWithOneElementList(opts ...TipBlockHeaderOption)
	OnTipBlockHeaderWillRespondWithEmptyList()
	WillBeUnreachable() error
	WillRespondWithInternalFailure()
	WillRespondWithMerklePath(status int, txID string, responseBody string)
	WillRespondWithBlockHeader(status int, blockHash string, responseBody string)
	WhenQueryingMerklePath(txID string) WhatsOnChainMerklePathQueryFixture
	WhenQueryingBlockHeader(blockHash string) WhatsOnChainBlockHeaderQueryFixture
	WillRespondWithBroadcast(status int, responseBody string)
	WillRespondOnTxStatus(status int, tc TxStatusExpectation)
	WillAlwaysReturnPostBEEFSuccess(txids ...string)
	Transport() *httpmock.MockTransport
	HttpClient() *resty.Client

	WillRespondWithConfirmedScriptHistory(status int, scriptHash string, response interface{})
	WillRespondWithUnconfirmedScriptHistory(status int, scriptHash string, response interface{})
	WillRespondWithScriptHistoryError(status int, scriptHash string, errorMsg string)
	WhenQueryingScriptHistory(scriptHash string) WhatsOnChainScriptHistoryQueryFixture
	WithValidScriptHistoryData() ScriptHistoryDataBuilder
	WithScriptHistoryValidationError(scriptHash string, expectedError string)
}

type wocFixture struct {
	testing.TB
	transport *httpmock.MockTransport
	network   defs.BSVNetwork
}

func NewWoCFixture(t testing.TB, opts ...Option) WhatsOnChainFixture {
	options := to.OptionsWithDefault(FixtureOptions{
		network:   defs.NetworkMainnet,
		transport: httpmock.NewMockTransport(),
	}, opts...)

	return &wocFixture{
		TB:        t,
		transport: options.transport,
		network:   options.network,
	}
}

func (f *wocFixture) WillRespondWithInternalFailure() {
	f.TB.Helper()
	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/headers?limit=1", f.network),
		httpmock.NewJsonResponderOrPanic(http.StatusInternalServerError, map[string]string{
			"error": http.StatusText(http.StatusInternalServerError),
		}),
	)
}

func (f *wocFixture) OnTipBlockHeaderWillRespondWithEmptyList() {
	f.TB.Helper()
	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/headers?limit=1", f.network),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []wocBlockResponseItem{}),
	)
}

type TipBlockHeaderOptions struct {
	Height uint
}

type TipBlockHeaderOption = func(*TipBlockHeaderOptions)

func WithTipBlockHeaderHeight(height uint) TipBlockHeaderOption {
	return func(opts *TipBlockHeaderOptions) {
		opts.Height = height
	}
}

func (f *wocFixture) OnTipBlockHeaderWillRespondWithOneElementList(opts ...TipBlockHeaderOption) {
	f.TB.Helper()

	options := to.OptionsWithDefault(TipBlockHeaderOptions{
		Height: TestBlockHeight,
	}, opts...)

	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/headers?limit=1", f.network),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []wocBlockResponseItem{
			{
				Hash:              TestBlockHash,
				Confirmations:     TestBlockConfirmations,
				Size:              TestBlockSize,
				Height:            options.Height,
				Version:           TestBlockVersion,
				VersionHex:        TestBlockVersionHex,
				MerkleRoot:        TestBlockMerkleRoot,
				Time:              TestBlockTime,
				MedianTime:        TestBlockMedianTime,
				Nonce:             TestBlockNonce,
				Bits:              TestBlockBits,
				Difficulty:        TestBlockDifficulty,
				ChainWork:         TestBlockChainWork,
				PreviousBlockHash: TestBlockPreviousBlockHash,
				NextBlockHash:     nil,
				NTx:               TestBlockNTx,
				NumTx:             TestBlockNumTx,
			},
		}),
	)
}

func (f *wocFixture) WillBeUnreachable() error {
	err := net.UnknownNetworkError("tests defined this endpoint as unreachable")
	f.TB.Helper()
	f.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`^https://api\.whatsonchain\.com/.*`),
		httpmock.NewErrorResponder(err),
	)
	return err
}

func (f *wocFixture) WillRespondWithRates(status int, content string, err error) {
	f.TB.Helper()
	responder := func(status int, content string) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (*http.Response, error) {
			if err != nil {
				return nil, err
			}
			res := httpmock.NewStringResponse(status, content)
			res.Header.Set("Content-Type", "application/json")
			return res, nil
		}
	}

	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/exchangerate", f.network),
		responder(status, content),
	)
}

func (f *wocFixture) WillRespondWithRawTx(status int, txID, rawTx string, err error) {
	f.TB.Helper()
	responder := func(status int, content string) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (*http.Response, error) {
			if err != nil {
				return nil, err
			}
			res := httpmock.NewStringResponse(status, content)
			res.Header.Set("Content-Type", "text/plain")
			return res, nil
		}
	}

	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/hex", f.network, txID),
		responder(status, rawTx),
	)
}

const (
	TestBlockConfirmations = 1
	TestBlockSize          = 2184411
	TestBlockHeight        = 901475
	TestBlockNTx           = 0
	TestBlockNumTx         = 3196
)

const (
	TestBlockVersion    uint32 = 805306368
	TestBlockNonce      uint32 = 602597547
	TestBlockTime       uint64 = 1750064695
	TestBlockMedianTime uint64 = 1750060569
)

const (
	TestBlockDifficulty = 64454475829.11144
)

const (
	TestBlockVersionHex        = "30000000"
	TestBlockMerkleRoot        = "c7a78f2edd611b0fe7aad6829a243e4a9e351e5ab203b7beb875ba1e6a80249e"
	TestBlockBits              = "18110ef8"
	TestBlockChainWork         = "000000000000000000000000000000000000000001669c7b159861f30c53271e"
	TestBlockPreviousBlockHash = "000000000000000001885e0c6c302cbbacf927e1b5cf7884588973e72f8b704e"
)

type wocBlockResponseItem struct {
	Hash              string  `json:"hash"`
	Confirmations     int     `json:"confirmations"`
	Size              int     `json:"size"`
	Height            uint    `json:"height"`
	Version           uint32  `json:"version"`
	VersionHex        string  `json:"versionHex"`
	MerkleRoot        string  `json:"merkleroot"`
	Time              uint64  `json:"time"`
	MedianTime        uint64  `json:"mediantime"`
	Nonce             uint32  `json:"nonce"`
	Bits              string  `json:"bits"`
	Difficulty        float64 `json:"difficulty"`
	ChainWork         string  `json:"chainwork"`
	PreviousBlockHash string  `json:"previousblockhash"`
	NextBlockHash     *string `json:"nextblockhash,omitempty"`
	NTx               int     `json:"nTx"`
	NumTx             int     `json:"num_tx"`
}

func (f *wocFixture) WillRespondWithMerklePath(status int, txID, responseBody string) {
	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseBody)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/proof/tsc", f.network, txID)
	f.transport.RegisterResponder("GET", url, responder)
}

func (f *wocFixture) WillRespondWithBlockHeader(status int, blockHash, responseBody string) {
	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseBody)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/%s/header", f.network, blockHash)
	f.transport.RegisterResponder("GET", url, responder)
}

func (f *wocFixture) WhenQueryingMerklePath(txID string) WhatsOnChainMerklePathQueryFixture {
	return &wocMerklePathQueryFixture{fixture: f, txID: txID}
}

func (f *wocFixture) WhenQueryingBlockHeader(blockHash string) WhatsOnChainBlockHeaderQueryFixture {
	return &wocBlockHeaderQueryFixture{fixture: f, blockHash: blockHash}
}

func (f *wocFixture) Transport() *httpmock.MockTransport {
	return f.transport
}

func (f *wocFixture) HttpClient() *resty.Client {
	client := resty.New()
	client.GetClient().Transport = f.transport
	return client
}

func (f *wocFixture) WillRespondWithBroadcast(status int, responseBody string) {
	responder := func(req *http.Request) (*http.Response, error) {
		res := httpmock.NewStringResponse(status, responseBody)
		res.Header.Set("Content-Type", "application/json")
		return res, nil
	}

	url := mockBroadcastURL(f.network)
	f.transport.RegisterResponder("POST", url, responder)
}

func (f *wocFixture) WillAlwaysReturnPostBEEFSuccess(txids ...string) {
	f.Transport().RegisterResponder("POST", mockBroadcastURL(f.network), func(req *http.Request) (*http.Response, error) {
		var body struct {
			TxHex string `json:"txhex"`
		}
		err := json.NewDecoder(req.Body).Decode(&body)
		if err != nil {
			return httpmock.NewStringResponse(http.StatusBadRequest, "bad request"), nil
		}

		rawTx, err := hex.DecodeString(body.TxHex)
		if err != nil {
			return httpmock.NewStringResponse(http.StatusBadRequest, "invalid hex"), nil
		}

		computedTxid := computeTxID(rawTx)

		for _, txid := range txids {
			if txid == computedTxid {
				respBody := fmt.Sprintf(`{"txid":"%s"}`, txid)
				resp := httpmock.NewStringResponse(http.StatusOK, respBody)
				resp.Header.Set("Content-Type", "application/json")
				return resp, nil
			}
		}

		return httpmock.NewStringResponse(http.StatusBadRequest, "txid not found"), nil
	})
}

type TxStatusExpectation struct {
	ExpectBlockHash   string
	ExpectBlockHeight int64
}

func (f *wocFixture) WillRespondOnTxStatus(status int, tc TxStatusExpectation) {
	f.TB.Helper()

	f.transport.RegisterResponder("POST",
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/txs/status", f.network),
		func(req *http.Request) (*http.Response, error) {
			var body struct {
				Txids []string `json:"txids"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(http.StatusBadRequest, "bad request"), nil
			}

			respItems := []map[string]interface{}{}
			for _, txid := range body.Txids {
				respItems = append(respItems, map[string]interface{}{
					"txid":          txid,
					"blockhash":     tc.ExpectBlockHash,
					"blockheight":   tc.ExpectBlockHeight,
					"confirmations": 10,
					"time":          1599999999,
					"blocktime":     1599999999,
				})
			}

			respBytes, _ := json.Marshal(respItems)
			resp := httpmock.NewStringResponse(status, string(respBytes))
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		})
}

func mockBroadcastURL(network defs.BSVNetwork) string {
	return fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/raw", network)
}

// computeTxID takes raw transaction bytes and returns the transaction ID (txid) as string.
func computeTxID(rawTx []byte) string {
	tx, err := transaction.NewTransactionFromBytes(rawTx)
	if err != nil {
		return ""
	}
	return tx.TxID().String()
}

type WhatsOnChainScriptHistoryQueryFixture interface {
	WillReturnConfirmedHistory(status int, response wdk.ScriptHashHistoryResponse)
	WillReturnUnconfirmedHistory(status int, response wdk.ScriptHashHistoryResponse)
	WillReturnConfirmedHistoryWithPagination(status int, response wdk.ScriptHashHistoryResponse, opts *wdk.GetConfirmedScriptHistoryOpts)
	WillReturnAPIError(errorMsg string)
	WillReturnHTTPError(status int)
}

type ScriptHistoryDataBuilder interface {
	WithConfirmedTransactions(count int, startHeight int) ScriptHistoryDataBuilder
	WithUnconfirmedTransactions(count int) ScriptHistoryDataBuilder
	WithEmptyHistory() ScriptHistoryDataBuilder
	WithScriptHash(scriptHash string) ScriptHistoryDataBuilder
	Build() (confirmedResponse, unconfirmedResponse wdk.ScriptHashHistoryResponse)
	SetupMocks(fixture WhatsOnChainFixture)
}

func (f *wocFixture) WillRespondWithConfirmedScriptHistory(status int, scriptHash string, response interface{}) {
	f.TB.Helper()
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/script/%s/confirmed/history", f.network, scriptHash)

	if response != nil {
		f.transport.RegisterResponder(
			http.MethodGet,
			url,
			httpmock.NewJsonResponderOrPanic(status, response),
		)
	} else {
		f.transport.RegisterResponder(
			http.MethodGet,
			url,
			httpmock.NewStringResponder(status, ""),
		)
	}
}

func (f *wocFixture) WillRespondWithUnconfirmedScriptHistory(status int, scriptHash string, response interface{}) {
	f.TB.Helper()
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/script/%s/unconfirmed/history", f.network, scriptHash)

	if response != nil {
		f.transport.RegisterResponder(
			http.MethodGet,
			url,
			httpmock.NewJsonResponderOrPanic(status, response),
		)
	} else {
		f.transport.RegisterResponder(
			http.MethodGet,
			url,
			httpmock.NewStringResponder(status, ""),
		)
	}
}

func (f *wocFixture) WillRespondWithScriptHistoryError(status int, scriptHash string, errorMsg string) {
	f.TB.Helper()

	errorResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{},
		Error:  errorMsg,
	}

	f.WillRespondWithConfirmedScriptHistory(status, scriptHash, errorResponse)
}

func (f *wocFixture) WhenQueryingScriptHistory(scriptHash string) WhatsOnChainScriptHistoryQueryFixture {
	return &wocScriptHistoryQueryFixture{
		fixture:    f,
		scriptHash: scriptHash,
	}
}

func (f *wocFixture) WithValidScriptHistoryData() ScriptHistoryDataBuilder {
	return &scriptHistoryDataBuilder{
		fixture:          f,
		scriptHash:       "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832", // default valid hash
		confirmedCount:   3,
		unconfirmedCount: 2,
		startHeight:      800000,
	}
}

func (f *wocFixture) WithScriptHistoryValidationError(scriptHash string, expectedError string) {
	f.TB.Helper()
}

type wocScriptHistoryQueryFixture struct {
	fixture    *wocFixture
	scriptHash string
}

func (q *wocScriptHistoryQueryFixture) WillReturnConfirmedHistory(status int, response wdk.ScriptHashHistoryResponse) {
	q.fixture.WillRespondWithConfirmedScriptHistory(status, q.scriptHash, response)
}

func (q *wocScriptHistoryQueryFixture) WillReturnUnconfirmedHistory(status int, response wdk.ScriptHashHistoryResponse) {
	q.fixture.WillRespondWithUnconfirmedScriptHistory(status, q.scriptHash, response)
}

func (q *wocScriptHistoryQueryFixture) WillReturnConfirmedHistoryWithPagination(status int, response wdk.ScriptHashHistoryResponse, opts *wdk.GetConfirmedScriptHistoryOpts) {
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/script/%s/confirmed/history", q.fixture.network, q.scriptHash)

	if opts != nil {
		params := make([]string, 0)

		if opts.Height != nil {
			params = append(params, fmt.Sprintf("height=%d", *opts.Height))
		}
		if opts.Limit != nil {
			params = append(params, fmt.Sprintf("limit=%d", *opts.Limit))
		}
		if opts.Order != nil {
			params = append(params, fmt.Sprintf("order=%s", opts.Order.String()))
		}
		if opts.NextPageToken != nil && *opts.NextPageToken != "" {
			params = append(params, fmt.Sprintf("token=%s", *opts.NextPageToken))
		}

		if len(params) > 0 {
			url += "?" + strings.Join(params, "&")
		}
	}

	fmt.Println(url)

	q.fixture.transport.RegisterResponder(
		http.MethodGet,
		url,
		httpmock.NewJsonResponderOrPanic(status, response),
	)
}

func (q *wocScriptHistoryQueryFixture) WillReturnAPIError(errorMsg string) {
	errorResponse := wdk.ScriptHashHistoryResponse{
		Result: []wdk.ScriptHashHistoryItem{},
		Error:  errorMsg,
	}
	q.WillReturnConfirmedHistory(http.StatusOK, errorResponse)
}

func (q *wocScriptHistoryQueryFixture) WillReturnHTTPError(status int) {
	q.WillReturnConfirmedHistory(status, wdk.ScriptHashHistoryResponse{})
}

type scriptHistoryDataBuilder struct {
	fixture          *wocFixture
	scriptHash       string
	confirmedCount   int
	unconfirmedCount int
	startHeight      int
	emptyHistory     bool
}

func (b *scriptHistoryDataBuilder) WithConfirmedTransactions(count int, startHeight int) ScriptHistoryDataBuilder {
	b.confirmedCount = count
	b.startHeight = startHeight
	b.emptyHistory = false
	return b
}

func (b *scriptHistoryDataBuilder) WithUnconfirmedTransactions(count int) ScriptHistoryDataBuilder {
	b.unconfirmedCount = count
	b.emptyHistory = false
	return b
}

func (b *scriptHistoryDataBuilder) WithEmptyHistory() ScriptHistoryDataBuilder {
	b.emptyHistory = true
	b.confirmedCount = 0
	b.unconfirmedCount = 0
	return b
}

func (b *scriptHistoryDataBuilder) WithScriptHash(scriptHash string) ScriptHistoryDataBuilder {
	b.scriptHash = scriptHash
	return b
}

func (b *scriptHistoryDataBuilder) Build() (confirmedResponse, unconfirmedResponse wdk.ScriptHashHistoryResponse) {
	if b.emptyHistory {
		return wdk.ScriptHashHistoryResponse{
				Result: []wdk.ScriptHashHistoryItem{},
				Error:  "",
			}, wdk.ScriptHashHistoryResponse{
				Result: []wdk.ScriptHashHistoryItem{},
				Error:  "",
			}
	}

	confirmedItems := make([]wdk.ScriptHashHistoryItem, b.confirmedCount)
	for i := 0; i < b.confirmedCount; i++ {
		confirmedItems[i] = wdk.ScriptHashHistoryItem{
			TxID:   fmt.Sprintf("confirmed_tx_%064d", i),
			Height: to.Ptr(b.startHeight + i),
		}
	}

	unconfirmedItems := make([]wdk.ScriptHashHistoryItem, b.unconfirmedCount)
	for i := 0; i < b.unconfirmedCount; i++ {
		unconfirmedItems[i] = wdk.ScriptHashHistoryItem{
			TxID:   fmt.Sprintf("unconfirmed_tx_%054d", i),
			Height: nil, // unconfirmed
		}
	}

	confirmedResponse = wdk.ScriptHashHistoryResponse{
		Result: confirmedItems,
		Error:  "",
	}

	unconfirmedResponse = wdk.ScriptHashHistoryResponse{
		Result: unconfirmedItems,
		Error:  "",
	}

	return confirmedResponse, unconfirmedResponse
}

func (b *scriptHistoryDataBuilder) SetupMocks(fixture WhatsOnChainFixture) {
	confirmedResp, unconfirmedResp := b.Build()
	fixture.WillRespondWithConfirmedScriptHistory(http.StatusOK, b.scriptHash, confirmedResp)
	fixture.WillRespondWithUnconfirmedScriptHistory(http.StatusOK, b.scriptHash, unconfirmedResp)
}

func ValidScriptHashes() map[string]string {
	return map[string]string{
		"p2pkh_standard": "76a914389ffce9cd9ae88dcc0631e88a821ffdbe9bfe2688ac",
		"p2sh_standard":  "a914b7536c788d8ca2de4d867a2b5b02acef97f35aef87",
		"script_hash_32": "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832",
		"script_hash_20": "1234567890abcdef1234567890abcdef12345678",
	}
}

func InvalidScriptHashes() map[string]struct {
	Hash          string
	ExpectedError string
} {
	return map[string]struct {
		Hash          string
		ExpectedError string
	}{
		"empty": {
			Hash:          "",
			ExpectedError: "scripthash cannot be empty",
		},
		"too_short": {
			Hash:          "a914b7536c",
			ExpectedError: "invalid scripthash length: too short",
		},
		"too_long": {
			Hash:          "a914b7536c788d8ca2de4d867a2b5b02acef97f35aef488aca914b7536c788d8ca2de4d867a2b5b02acef97f35aef488ac",
			ExpectedError: "invalid scripthash length: too long",
		},
		"invalid_hex": {
			Hash:          "g914b7536c788d8ca2de4d867a2b5b02acef97f35aef488ac",
			ExpectedError: "invalid scripthash format",
		},
		"non_hex_chars": {
			Hash:          "a914b7536c788d8ca2de4d867a2b5b02acef97f35aef488@c",
			ExpectedError: "invalid scripthash format",
		},
		"whitespace": {
			Hash:          "a914b7536c788d8ca2de4d867a2b5b02acef97f35aef488 c",
			ExpectedError: "invalid scripthash format",
		},
	}
}

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
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

type WhatsOnChainFixture interface {
	WillRespondWithEmptyBlockHeight()
	WillRespondWithRates(status int, content string, err error)
	WillRespondWithRawTx(status int, txID, rawTx string, err error)
	OnTipBlockHeaderWillRespondWithOneElementList(opts ...TipBlockHeaderOption)
	OnTipBlockHeaderWillRespondWithEmptyList()
	WillBeUnreachable() error
	WillRespondWithInternalFailure()
	WillRespondWithMerkleRoot(root string)
	WillRespondWithMerklePath(status int, txID, responseBody string)
	WillRespondWithBlockHeader(status int, blockHash, responseBody string)
	WillRespondWithBlockHeaderByHeight(status int, height uint32, merkleRoot string)
	WhenQueryingMerklePath(txID string) WhatsOnChainMerklePathQueryFixture
	WhenQueryingBlockHeader(blockHash string) WhatsOnChainBlockHeaderQueryFixture
	WillRespondWithBroadcast(status int, responseBody string)
	WillRespondOnTxStatus(status int, tc TxStatusExpectation)
	WillAlwaysReturnPostBEEFSuccess(txids ...string)
	WillRespondWithChainInfo(status int, blocks uint32)
	WillReturnMalformedBlockHeader(blockHash string)
	WillRespondWithUtxoStatus(status int, scriptHash, responseJSON string)
	Transport() *httpmock.MockTransport
	HttpClient() *resty.Client

	WillRespondWithConfirmedScriptHistory(status int, scriptHash, responseJSON string)
	WillRespondWithUnconfirmedScriptHistory(status int, scriptHash, responseJSON string)
	WillRespondWithScriptHistoryError(status int, scriptHash, errorMsg string)
	WhenQueryingScriptHistory(scriptHash string) WhatsOnChainScriptHistoryQueryFixture
	ScriptHistoryData() ScriptHistoryDataBuilder
	WithScriptHistoryValidationError(scriptHash, expectedError string)
	MinedTransaction() MinedTransactionFixture
}

type wocFixture struct {
	testing.TB

	getBeefFixture *minedTransactionFixture
	transport      *httpmock.MockTransport
	network        defs.BSVNetwork
}

func (f *wocFixture) WillRespondWithMerkleRoot(root string) {
	f.Helper()
	f.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https://api.whatsonchain.com/v1/bsv/%s/block/.*/header`, f.network)),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, blockHeaderDTO{
			Version:           TestBlockVersion,
			PreviousBlockHash: TestBlockPreviousBlockHash,
			MerkleRoot:        root,
			Time:              uint32(TestBlockTime),
			Bits:              TestBlockBits,
			Nonce:             TestBlockNonce,
			Hash:              TestBlockHash,
		}),
	)
}

func NewWoCFixture(t testing.TB, opts ...Option) WhatsOnChainFixture {
	options := to.OptionsWithDefault(FixtureOptions{
		network:   defs.NetworkMainnet,
		transport: httpmock.NewMockTransport(),
	}, opts...)

	fixture := &wocFixture{
		TB:        t,
		transport: options.transport,
		network:   options.network,
	}

	fixture.getBeefFixture = newGetBeefFixture(t, fixture)
	return fixture
}

func (f *wocFixture) WillRespondWithInternalFailure() {
	f.Helper()
	f.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https://api.whatsonchain.com/v1/bsv/%s/.*`, f.network)),
		httpmock.NewJsonResponderOrPanic(http.StatusInternalServerError, map[string]string{
			"error": http.StatusText(http.StatusInternalServerError),
		}),
	)
}

func (f *wocFixture) OnTipBlockHeaderWillRespondWithEmptyList() {
	f.Helper()
	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/headers?limit=1", f.network),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []wocBlockResponseItem{}),
	)
}

type TipBlockHeaderOptions struct {
	Height uint
	Hash   string
}

type TipBlockHeaderOption = func(*TipBlockHeaderOptions)

func WithTipBlockHeaderHeight(height uint) TipBlockHeaderOption {
	return func(opts *TipBlockHeaderOptions) {
		opts.Height = height
	}
}

func WithTipBlockHeaderHash(hash string) TipBlockHeaderOption {
	return func(opts *TipBlockHeaderOptions) {
		opts.Hash = hash
	}
}

func (f *wocFixture) WillRespondWithEmptyBlockHeight() {
	f.Helper()
	f.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https://api.whatsonchain.com/v1/bsv/%s/block/.*/header`, f.network)),
		httpmock.NewStringResponder(http.StatusOK, "{}"),
	)
}

func (f *wocFixture) OnTipBlockHeaderWillRespondWithOneElementList(opts ...TipBlockHeaderOption) {
	f.Helper()

	options := to.OptionsWithDefault(TipBlockHeaderOptions{
		Height: TestBlockHeight,
		Hash:   TestBlockHash,
	}, opts...)

	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/headers?limit=1", f.network),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []wocBlockResponseItem{
			{
				Hash:              options.Hash,
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
	f.Helper()
	f.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`^https://api\.whatsonchain\.com/.*`),
		httpmock.NewErrorResponder(err),
	)
	return err
}

func (f *wocFixture) WillRespondWithRates(status int, content string, err error) {
	f.Helper()
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
		responder(status, content), //nolint:bodyclose // mock responder for test fixture, not an actual HTTP response
	)
}

func (f *wocFixture) WillRespondWithRawTx(status int, txID, rawTx string, err error) {
	f.Helper()
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
		responder(status, rawTx), //nolint:bodyclose // mock responder for test fixture, not an actual HTTP response
	)
}

func (f *wocFixture) WillReturnMalformedBlockHeader(blockHash string) {
	f.Helper()
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/%s/header", f.network, blockHash)
	f.transport.RegisterResponder(http.MethodGet, url,
		httpmock.NewStringResponder(http.StatusOK, `invalid-json`),
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
	TestNextBlockHash          = "000000001546f288e1540d55b0a6b70f86c3fe0b29ca39ec7878c41f1f16ec5d"
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
	f.transport.RegisterResponder(http.MethodGet, url, responder)
}

func (f *wocFixture) WillRespondWithBlockHeader(status int, blockHash, responseBody string) {
	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseBody)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/%s/header", f.network, blockHash)
	f.transport.RegisterResponder(http.MethodGet, url, responder)
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
	f.transport.RegisterResponder(http.MethodPost, url, responder)
}

func (f *wocFixture) WillAlwaysReturnPostBEEFSuccess(txids ...string) {
	f.Transport().RegisterResponder(http.MethodPost, mockBroadcastURL(f.network), func(req *http.Request) (*http.Response, error) {
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
	f.Helper()

	f.transport.RegisterResponder(http.MethodPost,
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

// WillRespondWithBlockHeaderByHeight registers responders for any URL / block/{height}/header
func (f *wocFixture) WillRespondWithBlockHeaderByHeight(status int, height uint32, merkleRoot string) {
	f.Helper()

	responder := httpmock.NewJsonResponderOrPanic(
		status,
		headerByHeightDTO{
			Hash:              TestBlockHash,
			Confirmations:     TestBlockConfirmations,
			Size:              TestBlockSize,
			Height:            height,
			Version:           TestBlockVersion,
			VersionHex:        TestBlockVersionHex,
			MerkleRoot:        merkleRoot,
			Time:              TestBlockTime,
			MedianTime:        TestBlockMedianTime,
			Nonce:             TestBlockNonce,
			Bits:              TestBlockBits,
			Difficulty:        TestBlockDifficulty,
			ChainWork:         TestBlockChainWork,
			PreviousBlockHash: TestBlockPreviousBlockHash,
			NextBlockHash:     TestNextBlockHash,
			NTx:               TestBlockNTx,
			NumTx:             TestBlockNumTx,
		},
	)

	host := "https://api.whatsonchain.com"

	prefixes := []string{
		fmt.Sprintf("/v1/bsv/%s", f.network),
		fmt.Sprintf("/v1/%s", f.network),
	}

	for _, p := range prefixes {
		pathOnly := fmt.Sprintf("%s/block/%d/header", p, height)
		absolute := host + pathOnly

		f.transport.RegisterResponder(http.MethodGet, pathOnly, responder)
		f.transport.RegisterResponder(http.MethodGet, absolute, responder)
	}

	rx := fmt.Sprintf(`=~^/v1(?:/bsv)?/%s/block/%d/header$`, f.network, height)
	f.transport.RegisterResponder(http.MethodGet, rx, responder)
}

func (f *wocFixture) WillRespondWithChainInfo(status int, blocks uint32) {
	f.Helper()

	body := map[string]any{"blocks": blocks}

	abs := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/chain/info", f.network)
	f.transport.RegisterResponder(http.MethodGet, abs,
		httpmock.NewJsonResponderOrPanic(status, body))

	f.transport.RegisterResponder(http.MethodGet, fmt.Sprintf(`=~^/v1/bsv/%s/chain/info$`, f.network), httpmock.NewJsonResponderOrPanic(status, body))
}

type WhatsOnChainScriptHistoryQueryFixture interface {
	WillReturnConfirmedHistory(status int, responseJSON string)
	WillReturnUnconfirmedHistory(status int, responseJSON string)
	WillReturnAPIError(errorMsg string)
	WillReturnHTTPError(status int)
}

type ScriptHistoryDataBuilder interface {
	WithConfirmedTransactions(count, startHeight int) ScriptHistoryDataBuilder
	WithUnconfirmedTransactions(count int) ScriptHistoryDataBuilder
	WithEmptyHistory() ScriptHistoryDataBuilder
	WithScriptHash(scriptHash string) ScriptHistoryDataBuilder
	WithConfirmedStatusCode(statusCode int) ScriptHistoryDataBuilder
	WithUnconfirmedStatusCode(statusCode int) ScriptHistoryDataBuilder
	WithConfirmedTransactionsError(errorMsg string) ScriptHistoryDataBuilder
	WithUnconfirmedTransactionsError(errorMsg string) ScriptHistoryDataBuilder
	WithConfirmedTransactionsNotFound() ScriptHistoryDataBuilder
	WithUnconfirmedTransactionsNotFound(errorMsg string) ScriptHistoryDataBuilder
	WithConfirmedTransactionsInternalError(errorMsg string) ScriptHistoryDataBuilder
	WithUnconfirmedTransactionsInternalError(errorMsg string) ScriptHistoryDataBuilder
	WillBeReturned()
}

func (f *wocFixture) WillRespondWithConfirmedScriptHistory(status int, scriptHash, responseJSON string) {
	f.Helper()
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/script/%s/confirmed/history", f.network, scriptHash)

	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseJSON)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}

	f.transport.RegisterResponder(http.MethodGet, url, responder)
}

func (f *wocFixture) WillRespondWithUnconfirmedScriptHistory(status int, scriptHash, responseJSON string) {
	f.Helper()
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/script/%s/unconfirmed/history", f.network, scriptHash)

	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseJSON)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}

	f.transport.RegisterResponder(http.MethodGet, url, responder)
}

func (f *wocFixture) WillRespondWithScriptHistoryError(status int, scriptHash, errorMsg string) {
	f.Helper()

	errorResponseJSON := fmt.Sprintf(`{
		"result": [],
		"error": "%s"
	}`, errorMsg)

	f.WillRespondWithConfirmedScriptHistory(status, scriptHash, errorResponseJSON)
}

func (f *wocFixture) WhenQueryingScriptHistory(scriptHash string) WhatsOnChainScriptHistoryQueryFixture {
	return &wocScriptHistoryQueryFixture{
		fixture:    f,
		scriptHash: scriptHash,
	}
}

func (f *wocFixture) WithScriptHistoryValidationError(scriptHash, expectedError string) {
	f.Helper()
}

type wocScriptHistoryQueryFixture struct {
	fixture    *wocFixture
	scriptHash string
}

func (q *wocScriptHistoryQueryFixture) WillReturnAPIError(errorMsg string) {
	errorResponseJSON := fmt.Sprintf(`{
		"result": [],
		"error": "%s"
	}`, errorMsg)
	q.WillReturnConfirmedHistory(http.StatusOK, errorResponseJSON)
}

func (q *wocScriptHistoryQueryFixture) WillReturnConfirmedHistory(status int, responseJSON string) {
	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseJSON)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/script/%s/confirmed/history", q.fixture.network, q.scriptHash)
	q.fixture.transport.RegisterResponder(http.MethodGet, url, responder)
}

func (q *wocScriptHistoryQueryFixture) WillReturnUnconfirmedHistory(status int, responseJSON string) {
	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseJSON)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/script/%s/unconfirmed/history", q.fixture.network, q.scriptHash)
	q.fixture.transport.RegisterResponder(http.MethodGet, url, responder)
}

func (q *wocScriptHistoryQueryFixture) WillReturnHTTPError(status int) {
	q.WillReturnConfirmedHistory(status, "")
}

type scriptHistoryDataBuilder struct {
	fixture               *wocFixture
	scriptHash            string
	confirmedCount        int
	unconfirmedCount      int
	startHeight           int
	emptyHistory          bool
	confirmedError        string
	unconfirmedError      string
	confirmedStatusCode   int
	unconfirmedStatusCode int
}

func (f *wocFixture) ScriptHistoryData() ScriptHistoryDataBuilder {
	return &scriptHistoryDataBuilder{
		fixture:               f,
		scriptHash:            "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832",
		confirmedCount:        3,
		unconfirmedCount:      2,
		startHeight:           800000,
		emptyHistory:          false,
		confirmedError:        "",
		unconfirmedError:      "",
		confirmedStatusCode:   http.StatusOK,
		unconfirmedStatusCode: http.StatusOK,
	}
}

func (b *scriptHistoryDataBuilder) WithConfirmedStatusCode(statusCode int) ScriptHistoryDataBuilder {
	b.confirmedStatusCode = statusCode
	return b
}

func (b *scriptHistoryDataBuilder) WithUnconfirmedStatusCode(statusCode int) ScriptHistoryDataBuilder {
	b.unconfirmedStatusCode = statusCode
	return b
}

func (b *scriptHistoryDataBuilder) WithConfirmedTransactionsError(errorMsg string) ScriptHistoryDataBuilder {
	b.confirmedError = errorMsg
	b.confirmedCount = 0
	b.emptyHistory = false
	return b
}

func (b *scriptHistoryDataBuilder) WithUnconfirmedTransactionsError(errorMsg string) ScriptHistoryDataBuilder {
	b.unconfirmedError = errorMsg
	b.unconfirmedCount = 0
	b.emptyHistory = false
	return b
}

func (b *scriptHistoryDataBuilder) WithConfirmedTransactionsNotFound() ScriptHistoryDataBuilder {
	return b.WithConfirmedTransactionsError("").WithConfirmedStatusCode(http.StatusNotFound)
}

func (b *scriptHistoryDataBuilder) WithUnconfirmedTransactionsNotFound(errorMsg string) ScriptHistoryDataBuilder {
	return b.WithUnconfirmedTransactionsError(errorMsg).WithUnconfirmedStatusCode(http.StatusNotFound)
}

func (b *scriptHistoryDataBuilder) WithConfirmedTransactionsInternalError(errorMsg string) ScriptHistoryDataBuilder {
	return b.WithConfirmedTransactionsError(errorMsg).WithConfirmedStatusCode(http.StatusInternalServerError)
}

func (b *scriptHistoryDataBuilder) WithUnconfirmedTransactionsInternalError(errorMsg string) ScriptHistoryDataBuilder {
	return b.WithUnconfirmedTransactionsError(errorMsg).WithUnconfirmedStatusCode(http.StatusInternalServerError)
}

func (b *scriptHistoryDataBuilder) WithConfirmedTransactions(count, startHeight int) ScriptHistoryDataBuilder {
	b.confirmedCount = count
	b.startHeight = startHeight
	b.emptyHistory = false
	b.confirmedError = ""
	b.confirmedStatusCode = http.StatusOK
	return b
}

func (b *scriptHistoryDataBuilder) WithUnconfirmedTransactions(count int) ScriptHistoryDataBuilder {
	b.unconfirmedCount = count
	b.emptyHistory = false
	b.unconfirmedError = ""
	b.unconfirmedStatusCode = http.StatusOK
	return b
}

func (b *scriptHistoryDataBuilder) WithEmptyHistory() ScriptHistoryDataBuilder {
	b.emptyHistory = true
	b.confirmedCount = 0
	b.unconfirmedCount = 0
	b.confirmedError = ""
	b.confirmedStatusCode = http.StatusOK
	return b
}

func (b *scriptHistoryDataBuilder) WithScriptHash(scriptHash string) ScriptHistoryDataBuilder {
	b.scriptHash = scriptHash
	return b
}

func (b *scriptHistoryDataBuilder) buildJSON() (confirmedJSON, unconfirmedJSON string) {
	if b.emptyHistory {
		return `{"result": [], "error": ""}`, `{"result": [], "error": ""}`
	}

	if b.confirmedError != "" {
		confirmedJSON = b.buildConfirmedError()
	} else {
		confirmedJSON = b.buildConfirmedSuccess()
	}

	if b.unconfirmedError != "" {
		unconfirmedJSON = b.buildUnconfirmedError()
	} else {
		unconfirmedJSON = b.buildUnconfirmedSuccess()
	}

	return confirmedJSON, unconfirmedJSON
}

func (b *scriptHistoryDataBuilder) buildConfirmedSuccess() string {
	confirmedItems := make([]string, b.confirmedCount)
	for i := 0; i < b.confirmedCount; i++ {
		confirmedItems[i] = fmt.Sprintf(`{
			"tx_hash": "c%010de1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9",
			"height": %d
		}`, i, b.startHeight+i)
	}

	return fmt.Sprintf(`{
		"result": [%s],
		"error": ""
	}`, strings.Join(confirmedItems, ","))
}

func (b *scriptHistoryDataBuilder) buildConfirmedError() string {
	return fmt.Sprintf(`{
		"result": [],
		"error": "%s"
	}`, b.confirmedError)
}

func (b *scriptHistoryDataBuilder) buildUnconfirmedSuccess() string {
	unconfirmedItems := make([]string, b.unconfirmedCount)
	for i := 0; i < b.unconfirmedCount; i++ {
		unconfirmedItems[i] = fmt.Sprintf(`{
			"tx_hash": "u%010de1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9",
			"height": null
		}`, i)
	}

	return fmt.Sprintf(`{
		"result": [%s],
		"error": ""
	}`, strings.Join(unconfirmedItems, ","))
}

func (b *scriptHistoryDataBuilder) buildUnconfirmedError() string {
	return fmt.Sprintf(`{
		"result": [],
		"error": "%s"
	}`, b.unconfirmedError)
}

func (b *scriptHistoryDataBuilder) WillBeReturned() {
	b.fixture.Helper()

	confirmedResp, unconfirmedResp := b.buildJSON()

	b.fixture.WillRespondWithConfirmedScriptHistory(b.confirmedStatusCode, b.scriptHash, confirmedResp)
	b.fixture.WillRespondWithUnconfirmedScriptHistory(b.unconfirmedStatusCode, b.scriptHash, unconfirmedResp)
}

func (f *wocFixture) WillRespondWithUtxoStatus(status int, scriptHash, responseJSON string) {
	f.Helper()
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/script/%s/unspent/all", f.network, scriptHash)
	responder := func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(status, responseJSON)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	f.transport.RegisterResponder(http.MethodGet, url, responder)
}

func (f *wocFixture) MinedTransaction() MinedTransactionFixture {
	return f.getBeefFixture
}

package testservices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

type BitailsFixture interface {
	WillRespondWithEmptyBlockHeight()
	WillBeUnreachable() error
	WillReturnInternalError()
	WillReturnTxInfo(txid, blockHash string, blockHeight int64)
	WillReturnSuccessAndTxInfo(txid, blockHash string, blockHeight int64)
	WillReturnTscProof(txid, target string, index int, nodes []string)
	WillReturnBlockHeader(blockHash, rawHeader string)
	WillReturnBranchProof(txid, blockHash, merkleRoot string, branches []map[string]string)
	WillReturnTxStatus(txid string, blockHeight int)
	WillRespondWithBlockHeaderByHeight(status int, height uint32, headerHex string)
	WillReturnNetworkInfo(status int, blocks uint32)
	WillReturnLatestBlock(blockHash string, height uint32)
	WillReturnRawTxHex(txid, rawHex string)
	WillReturnRawTx404(txid string)
	WillReturnRawTxHttpError(txid string, status int)
	WillRespondWithInternalFailure()
	WillReturnBlockHeaderHttpError(blockHash string, status int)
	WillReturnMalformedBlockHeader(blockHash string)
	WillReturnTxStatusNotFound(txid string)
	WillReturnTxStatusMined(txid string, height int)
	WillReturnTxStatusUnconfirmed(txid string)
	WillReturnTxStatusHttpError(txid string, status int)
	WillRespondWithBlockByHeight()
	ScriptHistoryData() ScriptHistoryDataBuilder
	OnBroadcast() BitailsBroadcastFixture
	HttpClient() *resty.Client
	Transport() *httpmock.MockTransport
}

type BitailsBroadcastFixture interface {
	WillReturnSuccess(string)
	WillReturnAlreadyInMempool(string, error)
	WillReturnDoubleSpend(string, error)
	WillReturnMissingInputs(string, error)
	WillReturnMalformedResponse()
	WillReturnHttpError(int)
	WillReturnEconnRefused(string, error)
	WillReturnEconnReset(string, error)
}

type bitailsFixture struct {
	testing.TB

	transport *httpmock.MockTransport
	network   defs.BSVNetwork
}

func NewBitailsFixture(t testing.TB, opts ...Option) BitailsFixture {
	options := to.OptionsWithDefault(FixtureOptions{
		network:   defs.NetworkMainnet,
		transport: httpmock.NewMockTransport(),
	}, opts...)

	return &bitailsFixture{
		TB:        t,
		transport: options.transport,
		network:   options.network,
	}
}

func (b *bitailsFixture) HttpClient() *resty.Client {
	client := resty.New()
	client.SetTransport(b.transport)
	return client
}

func (b *bitailsFixture) Transport() *httpmock.MockTransport {
	return b.transport
}

func (b *bitailsFixture) OnBroadcast() BitailsBroadcastFixture {
	return &bitailsBroadcastFixture{
		TB:        b.TB,
		transport: b.transport,
		network:   b.network,
	}
}

func (b *bitailsFixture) WillBeUnreachable() error {
	err := fmt.Errorf("bitails unreachable (test induced)")
	responder := httpmock.NewErrorResponder(err)

	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`https?://.*\.bitails\.io/.*`),
		responder,
	)
	b.transport.RegisterRegexpResponder(
		http.MethodPost,
		regexp.MustCompile(`https?://.*\.bitails\.io/.*`),
		responder,
	)
	return err
}

func (b *bitailsFixture) WillReturnInternalError() {
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`https?://.*\.bitails\.io/block/latest`),
		httpmock.NewStringResponder(http.StatusInternalServerError, "internal test error"),
	)

	b.transport.RegisterRegexpResponder(
		http.MethodPost,
		regexp.MustCompile(`https?://.*\.bitails\.io.*`),
		httpmock.NewJsonResponderOrPanic(http.StatusInternalServerError, map[string]string{
			"error": http.StatusText(http.StatusInternalServerError),
		}),
	)
}

func (b *bitailsFixture) WillReturnTxInfo(txid, blockHash string, blockHeight int64) {
	body := map[string]any{
		"block_hash":   blockHash,
		"block_height": blockHeight,
	}
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/tx/%s/status`, regexp.QuoteMeta(txid))),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, body),
	)
}

func (b *bitailsFixture) WillReturnSuccessAndTxInfo(txid, blockHash string, blockHeight int64) {
	b.WillReturnTxInfo(txid, blockHash, blockHeight)
	b.OnBroadcast().WillReturnSuccess(txid)
}

type bitailsBroadcastFixture struct {
	testing.TB

	transport *httpmock.MockTransport
	network   defs.BSVNetwork
}

func (b *bitailsBroadcastFixture) WillReturnSuccess(txid string) {
	body := []map[string]any{
		{"txid": txid},
	}
	b.registerBroadcastResponder(body)
}

func (b *bitailsBroadcastFixture) WillReturnAlreadyInMempool(txid string, err error) {
	b.registerBroadcastResponder([]map[string]any{
		{
			"txid": txid,
			"error": map[string]any{
				"code":    -27,
				"message": err.Error(),
			},
		},
	})
}

func (b *bitailsBroadcastFixture) WillReturnDoubleSpend(txid string, err error) {
	b.registerBroadcastResponder([]map[string]any{
		{
			"txid": txid,
			"error": map[string]any{
				"code":    -26,
				"message": err.Error(),
			},
		},
	})
}

func (b *bitailsBroadcastFixture) WillReturnMissingInputs(txid string, err error) {
	b.registerBroadcastResponder([]map[string]any{
		{
			"txid": txid,
			"error": map[string]any{
				"code":    -25,
				"message": err.Error(),
			},
		},
	})
}

func (b *bitailsBroadcastFixture) WillReturnMalformedResponse() {
	b.registerBroadcastResponder(map[string]any{"malformed": true})
}

func (b *bitailsBroadcastFixture) WillReturnHttpError(status int) {
	b.transport.RegisterRegexpResponder(
		http.MethodPost,
		regexp.MustCompile(`https?://.*\.bitails\.io/tx/broadcast/multi`),
		httpmock.NewStringResponder(status, "internal test error"),
	)
}

func (b *bitailsBroadcastFixture) WillReturnEconnRefused(txid string, err error) {
	b.registerBroadcastResponder([]map[string]any{
		{
			"txid": txid,
			"error": map[string]any{
				"code":    "ECONNREFUSED",
				"errno":   -111,
				"message": err.Error(),
			},
		},
	})
}

func (b *bitailsBroadcastFixture) WillReturnEconnReset(txid string, err error) {
	b.registerBroadcastResponder([]map[string]any{
		{
			"txid": txid,
			"error": map[string]any{
				"code":    "ECONNRESET",
				"errno":   -104,
				"message": err.Error(),
			},
		},
	})
}

func (b *bitailsBroadcastFixture) registerBroadcastResponder(body any) {
	data, err := json.Marshal(body)
	if err != nil {
		b.Fatalf("failed to marshal broadcast response: %v", err)
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	responder := httpmock.NewStringResponder(http.StatusCreated, string(data)).
		HeaderSet(headers)

	b.transport.RegisterRegexpResponder(
		http.MethodPost,
		regexp.MustCompile(`https?://.*\.bitails\.io/tx/broadcast/multi`),
		responder,
	)
}

func (b *bitailsFixture) WillReturnTscProof(txid, target string, index int, nodes []string) {
	body := map[string]any{
		"index":  index,
		"txOrId": txid,
		"target": target,
		"nodes":  nodes,
	}
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/tx/%s/proof/tsc`, regexp.QuoteMeta(txid))),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, body),
	)
}

func (b *bitailsFixture) WillReturnBlockHeader(blockHash, rawHeader string) {
	pattern := fmt.Sprintf(`^https://api\.bitails\.io/block/%s/header$`, regexp.QuoteMeta(blockHash))
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(pattern),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]any{
			"header": rawHeader,
		}),
	)
}

func (b *bitailsFixture) WillReturnBranchProof(txid, blockHash, merkleRoot string, branches []map[string]string) {
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/tx/%s/proof(?:\?.*)?(?:/.*)?$`, regexp.QuoteMeta(txid))),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]any{
			"blockhash":  blockHash,
			"merkleRoot": merkleRoot,
			"branches":   branches,
		}),
	)
}

func (b *bitailsFixture) WillReturnTxStatus(txid string, blockHeight int) {
	body := map[string]any{
		"blockHeight": blockHeight,
	}
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/tx/%s/status`, regexp.QuoteMeta(txid))),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, body),
	)
}

func (b *bitailsFixture) WillRespondWithEmptyBlockHeight() {
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`https?://.*\.bitails\.io/block/height/.*`),
		httpmock.NewStringResponder(http.StatusOK, "{}"),
	)
}

func (b *bitailsFixture) WillRespondWithBlockByHeight() {
	b.Helper()
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`https?://.*\.bitails\.io/block/height/.*`),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]any{
			"previousBlockHash": TestBlockPreviousBlockHash,
			"version":           TestBlockVersion,
			"time":              TestBlockTime,
			"bits":              TestBlockBits,
			"nonce":             TestBlockNonce,
			"merkleRoot":        TestBlockMerkleRoot,
			"hash":              TestBlockHash,
		}),
	)
}

func (b *bitailsFixture) WillRespondWithBlockHeaderByHeight(status int, height uint32, headerHex string) {
	pattern := `=~.*?/block/header/height/` + strconv.Itoa(int(height)) + `/raw$`

	var responder httpmock.Responder
	switch status {
	case http.StatusOK:
		responder = httpmock.NewJsonResponderOrPanic(status, struct {
			Header string `json:"header"`
		}{Header: headerHex})
	default:
		responder = httpmock.NewStringResponder(status, http.StatusText(status))
	}

	b.transport.RegisterResponder(http.MethodGet, pattern, responder)
}

func (b *bitailsFixture) WillReturnNetworkInfo(status int, blocks uint32) {
	b.Helper()

	body := map[string]any{"blocks": blocks}
	pat := `=~.*?/network/info$`
	b.transport.RegisterResponder(http.MethodGet, pat, httpmock.NewJsonResponderOrPanic(status, body))
}

// WillReturnLatestBlock stubs GET /block/latest.
func (b *bitailsFixture) WillReturnLatestBlock(blockHash string, height uint32) {
	body := map[string]any{"hash": blockHash, "height": height}

	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`https?://.*\.bitails\.io/block/latest`),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, body),
	)
}

// WillRespondWithInternalFailure forces GET /block/latest to reply 500.
func (b *bitailsFixture) WillRespondWithInternalFailure() {
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`https?://.*\.bitails\.io/block/latest`),
		httpmock.NewStringResponder(http.StatusInternalServerError, "internal test error"),
	)

	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`https?://.*\.bitails\.io/block/height/.*`),
		httpmock.NewStringResponder(http.StatusInternalServerError, "internal test error"),
	)
}

func (b *bitailsFixture) WillReturnRawTxHex(txid, rawHex string) {
	pattern := fmt.Sprintf(`https?://.*\.bitails\.io/download/tx/%s/hex`, regexp.QuoteMeta(txid))
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(pattern),
		httpmock.NewStringResponder(http.StatusOK, rawHex),
	)
}

func (b *bitailsFixture) WillReturnRawTx404(txid string) {
	pattern := fmt.Sprintf(`https?://.*\.bitails\.io/download/tx/%s/hex`, regexp.QuoteMeta(txid))
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(pattern),
		httpmock.NewStringResponder(http.StatusNotFound, "not found"),
	)
}

func (b *bitailsFixture) WillReturnRawTxHttpError(txid string, status int) {
	pattern := fmt.Sprintf(`https?://.*\.bitails\.io/download/tx/%s/hex`, regexp.QuoteMeta(txid))
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(pattern),
		httpmock.NewStringResponder(status, http.StatusText(status)),
	)
}

func (b *bitailsFixture) WillReturnBlockHeaderHttpError(blockHash string, status int) {
	pattern := fmt.Sprintf(`https?://.*\.bitails\.io/block/%s/header`, regexp.QuoteMeta(blockHash))
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(pattern),
		httpmock.NewStringResponder(status, "internal test error"),
	)
}

func (b *bitailsFixture) WillReturnMalformedBlockHeader(blockHash string) {
	pattern := fmt.Sprintf(`https?://.*\.bitails\.io/block/%s/header`, regexp.QuoteMeta(blockHash))
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(pattern),
		httpmock.NewStringResponder(http.StatusOK, `invalid-json}`),
	)
}

func (b *bitailsFixture) ScriptHistoryData() ScriptHistoryDataBuilder {
	return &bitailsScriptHistoryBuilder{
		fixture:          b,
		scriptHash:       "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832",
		confirmedCount:   2,
		unconfirmedCount: 1,
		startHeight:      800000,
	}
}

type bitailsScriptHistoryBuilder struct {
	fixture               *bitailsFixture
	scriptHash            string
	confirmedCount        int
	unconfirmedCount      int
	startHeight           int
	apiError              string
	confirmedStatusCode   int
	unconfirmedStatusCode int
}

func (b *bitailsScriptHistoryBuilder) WithConfirmedTransactions(count, startHeight int) ScriptHistoryDataBuilder {
	b.confirmedCount = count
	b.startHeight = startHeight
	return b
}

func (b *bitailsScriptHistoryBuilder) WithUnconfirmedTransactions(count int) ScriptHistoryDataBuilder {
	b.unconfirmedCount = count
	return b
}

func (b *bitailsScriptHistoryBuilder) WithEmptyHistory() ScriptHistoryDataBuilder {
	b.confirmedCount = 0
	b.unconfirmedCount = 0
	return b
}

func (b *bitailsScriptHistoryBuilder) WithScriptHash(scriptHash string) ScriptHistoryDataBuilder {
	b.scriptHash = scriptHash
	return b
}

func (b *bitailsScriptHistoryBuilder) WithConfirmedStatusCode(statusCode int) ScriptHistoryDataBuilder {
	b.confirmedStatusCode = statusCode
	return b
}

func (b *bitailsScriptHistoryBuilder) WithUnconfirmedStatusCode(statusCode int) ScriptHistoryDataBuilder {
	b.unconfirmedStatusCode = statusCode
	return b
}

func (b *bitailsScriptHistoryBuilder) WithConfirmedTransactionsError(errorMsg string) ScriptHistoryDataBuilder {
	b.apiError = errorMsg
	b.confirmedCount = 0
	return b
}

func (b *bitailsScriptHistoryBuilder) WithUnconfirmedTransactionsError(errorMsg string) ScriptHistoryDataBuilder {
	b.apiError = errorMsg
	b.unconfirmedCount = 0
	return b
}

func (b *bitailsScriptHistoryBuilder) WithConfirmedTransactionsNotFound() ScriptHistoryDataBuilder {
	b.confirmedStatusCode = http.StatusNotFound
	return b
}

func (b *bitailsScriptHistoryBuilder) WithUnconfirmedTransactionsNotFound(errorMsg string) ScriptHistoryDataBuilder {
	b.unconfirmedStatusCode = http.StatusNotFound
	b.apiError = errorMsg
	return b
}

func (b *bitailsScriptHistoryBuilder) WithConfirmedTransactionsInternalError(errorMsg string) ScriptHistoryDataBuilder {
	b.confirmedStatusCode = http.StatusInternalServerError
	b.apiError = errorMsg
	return b
}

func (b *bitailsScriptHistoryBuilder) WithUnconfirmedTransactionsInternalError(errorMsg string) ScriptHistoryDataBuilder {
	b.unconfirmedStatusCode = http.StatusInternalServerError
	b.apiError = errorMsg
	return b
}

func (b *bitailsScriptHistoryBuilder) WillBeReturned() {
	b.fixture.Helper()

	type ScriptHistoryItem struct {
		TxID   string `json:"txid"`
		Height *int   `json:"blockheight,omitempty"`
	}
	type ScriptHistoryResponse struct {
		ScriptHash string              `json:"scripthash,omitempty"`
		History    []ScriptHistoryItem `json:"history"`
		PgKey      string              `json:"pgkey,omitempty"`
		Error      string              `json:"error,omitempty"`
	}

	var response ScriptHistoryResponse
	response.ScriptHash = b.scriptHash

	if b.apiError != "" {
		response.Error = b.apiError
	} else {
		for i := 0; i < b.confirmedCount; i++ {
			height := b.startHeight + i
			response.History = append(response.History, ScriptHistoryItem{
				TxID:   fmt.Sprintf("%02x%062s", i, "e1b71dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9"),
				Height: &height,
			})
		}
		for i := 0; i < b.unconfirmedCount; i++ {
			txid := fmt.Sprintf("%02x%062s", i, "e1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9")
			response.History = append(response.History, ScriptHistoryItem{
				TxID:   txid,
				Height: nil,
			})
		}
	}

	statusCode := http.StatusOK
	if b.confirmedStatusCode != 0 {
		statusCode = b.confirmedStatusCode
	}

	b.fixture.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`/scripthash/`+regexp.QuoteMeta(b.scriptHash)+`/history(?:\?.*)?$`),
		httpmock.NewJsonResponderOrPanic(statusCode, response),
	)
}

func (b *bitailsFixture) WillReturnTxStatusNotFound(txid string) {
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/tx/%s/status`, regexp.QuoteMeta(txid))),
		httpmock.NewStringResponder(http.StatusNotFound, "not found"),
	)
}

func (b *bitailsFixture) WillReturnTxStatusHttpError(txid string, status int) {
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/tx/%s/status`, regexp.QuoteMeta(txid))),
		httpmock.NewStringResponder(status, http.StatusText(status)),
	)
}

func (b *bitailsFixture) WillReturnTxStatusMined(txid string, height int) {
	body := map[string]any{
		"blockhash":   "some-block-hash",
		"blockheight": height,
	}
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/tx/%s/status`, regexp.QuoteMeta(txid))),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, body),
	)
}

func (b *bitailsFixture) WillReturnTxStatusUnconfirmed(txid string) {
	body := map[string]any{
		"blockhash":   "",
		"blockheight": 0,
	}
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/tx/%s/status`, regexp.QuoteMeta(txid))),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, body),
	)
}

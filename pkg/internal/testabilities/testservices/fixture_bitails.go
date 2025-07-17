package testservices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
)

type BitailsFixture interface {
	WillBeUnreachable() error
	WillReturnInternalError()
	WillReturnTxInfo(txid string, blockHash string, blockHeight int64)
	WillReturnSuccessAndTxInfo(txid string, blockHash string, blockHeight int64)
	WillReturnTscProof(txid, target string, index int, nodes []string)
	WillReturnBlockHeader(blockHash, rawHeader string)
	WillReturnBranchProof(txid, blockHash, merkleRoot string, branches []map[string]string)
	WillReturnTxStatus(txid string, blockHeight int)
	WillRespondWithBlockHeaderByHeight(status int, height uint32, headerHex string)
	WillReturnNetworkInfo(status int, blocks uint32)
	WillReturnLatestBlock(blockHash string, height uint32)
	WillRespondWithInternalFailure()
	OnBroadcast() BitailsBroadcastFixture
	HttpClient() *resty.Client
	Transport() *httpmock.MockTransport
}

type BitailsBroadcastFixture interface {
	WillReturnSuccess(string)
	WillReturnAlreadyInMempool(string, error)
	WillReturnDoubleSpend(string, error)
	WillReturnMalformedResponse()
	WillReturnHttpError(int)
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

func (b *bitailsFixture) WillReturnTxInfo(txid string, blockHash string, blockHeight int64) {
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

func (b *bitailsFixture) WillReturnSuccessAndTxInfo(txid string, blockHash string, blockHeight int64) {
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
	b.registerBroadcastResponder(http.StatusCreated, body)
}

func (b *bitailsBroadcastFixture) WillReturnAlreadyInMempool(txid string, err error) {
	b.registerBroadcastResponder(http.StatusCreated, []map[string]any{
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
	b.registerBroadcastResponder(http.StatusCreated, []map[string]any{
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
	b.registerBroadcastResponder(http.StatusCreated, map[string]any{"malformed": true})
}

func (b *bitailsBroadcastFixture) WillReturnHttpError(status int) {
	b.transport.RegisterRegexpResponder(
		http.MethodPost,
		regexp.MustCompile(`https?://.*\.bitails\.io/tx/broadcast/multi`),
		httpmock.NewStringResponder(status, "internal test error"),
	)
}

func (b *bitailsBroadcastFixture) registerBroadcastResponder(status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		b.TB.Fatalf("failed to marshal broadcast response: %v", err)
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	responder := httpmock.NewStringResponder(status, string(data)).
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
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/block/%s/header`, regexp.QuoteMeta(blockHash))),
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

func (f *bitailsFixture) WillRespondWithBlockHeaderByHeight(status int, height uint32, headerHex string) {
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

	f.transport.RegisterResponder(http.MethodGet, pattern, responder)
}

func (b *bitailsFixture) WillReturnNetworkInfo(status int, blocks uint32) {
	b.TB.Helper()

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
}

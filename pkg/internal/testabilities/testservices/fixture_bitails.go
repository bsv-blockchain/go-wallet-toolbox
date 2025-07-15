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

func (f *bitailsFixture) HttpClient() *resty.Client {
	client := resty.New()
	client.SetTransport(f.transport)
	return client
}

func (f *bitailsFixture) Transport() *httpmock.MockTransport {
	return f.transport
}

func (f *bitailsFixture) OnBroadcast() BitailsBroadcastFixture {
	return &bitailsBroadcastFixture{
		TB:        f.TB,
		transport: f.transport,
		network:   f.network,
	}
}

func (f *bitailsFixture) WillBeUnreachable() error {
	err := fmt.Errorf("bitails unreachable (test induced)")
	f.transport.RegisterRegexpResponder(
		http.MethodPost,
		regexp.MustCompile(`https?://.*\.bitails\.io.*`),
		httpmock.NewErrorResponder(err),
	)
	return err
}

func (f *bitailsFixture) WillReturnInternalError() {
	f.transport.RegisterRegexpResponder(
		http.MethodPost,
		regexp.MustCompile(`https?://.*\.bitails\.io.*`),
		httpmock.NewJsonResponderOrPanic(http.StatusInternalServerError, map[string]string{
			"error": http.StatusText(http.StatusInternalServerError),
		}),
	)
}

func (f *bitailsFixture) WillReturnTxInfo(txid string, blockHash string, blockHeight int64) {
	body := map[string]any{
		"block_hash":   blockHash,
		"block_height": blockHeight,
	}
	f.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/tx/%s/status`, regexp.QuoteMeta(txid))),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, body),
	)
}

func (f *bitailsFixture) WillReturnSuccessAndTxInfo(txid string, blockHash string, blockHeight int64) {
	f.WillReturnTxInfo(txid, blockHash, blockHeight)
	f.OnBroadcast().WillReturnSuccess(txid)
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

func (f *bitailsFixture) WillReturnTscProof(txid, target string, index int, nodes []string) {
	body := map[string]any{
		"index":  index,
		"txOrId": txid,
		"target": target,
		"nodes":  nodes,
	}
	f.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/tx/%s/proof/tsc`, regexp.QuoteMeta(txid))),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, body),
	)
}

func (f *bitailsFixture) WillReturnBlockHeader(blockHash, rawHeader string) {
	f.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/block/%s/header`, regexp.QuoteMeta(blockHash))),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]any{
			"header": rawHeader,
		}),
	)
}

func (f *bitailsFixture) WillReturnBranchProof(txid, blockHash, merkleRoot string, branches []map[string]string) {
	f.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`https?://.*\.bitails\.io/tx/%s/proof(?:\?.*)?(?:/.*)?$`, regexp.QuoteMeta(txid))),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]any{
			"blockhash":  blockHash,
			"merkleRoot": merkleRoot,
			"branches":   branches,
		}),
	)
}

func (f *bitailsFixture) WillReturnTxStatus(txid string, blockHeight int) {
	body := map[string]any{
		"blockHeight": blockHeight,
	}
	f.transport.RegisterRegexpResponder(
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

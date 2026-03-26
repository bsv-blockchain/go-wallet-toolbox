package testservices

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

const (
	tipLongestPath       = defs.BHSTestURL + "/api/v1/chain/tip/longest"
	verifyMerkleRootPath = defs.BHSTestURL + "/api/v1/chain/merkleroot/verify"
	headerByHeightPath   = defs.BHSTestURL + "/api/v1/chain/header/byHeight"
)

type LongestChainTipOptions func(*longestChainTipResponse)

func WithLongestChainTipHeight(h uint) LongestChainTipOptions {
	return func(l *longestChainTipResponse) {
		l.Height = h
	}
}

type BHSFixture interface {
	IsUpAndRunning() BHSFixture
	WillBeUnreachable() error
	WillRespondWithInternalFailure()
	WillRespondWithEmptyLongestTipBlockHeader()
	WillRespondWithEmptyBlockHeight()
	OnLongestTipBlockHeaderResponseWith(opts ...LongestChainTipOptions)
	OnMerkleRootVerifyResponse(height uint32, root, state string)
	DefaultLongestTip() *longestChainTipResponse
	DefaultHeaderByHeightResponse() *headerByHeightResponse
	HttpClient() *resty.Client
	Transport() *httpmock.MockTransport
}

type bhsFixture struct {
	testing.TB

	transport                  *httpmock.MockTransport
	longestChainTip            *longestChainTipResponse
	headerByHeightResponse     *headerByHeightResponse
	bhsAnyEndpointRegexFixture *regexp.Regexp
}

func (b *bhsFixture) DefaultHeaderByHeightResponse() *headerByHeightResponse {
	b.Helper()
	return b.headerByHeightResponse
}

func (b *bhsFixture) DefaultLongestTip() *longestChainTipResponse {
	b.Helper()
	return b.longestChainTip
}

func (b *bhsFixture) WillRespondWithEmptyLongestTipBlockHeader() {
	b.Helper()
	b.transport.RegisterResponder(
		http.MethodGet,
		tipLongestPath,
		httpmock.NewStringResponder(http.StatusOK, "{}"),
	)
}

func (b *bhsFixture) WillRespondWithEmptyBlockHeight() {
	b.Helper()
	b.transport.RegisterResponder(
		http.MethodGet,
		headerByHeightPath,
		httpmock.NewStringResponder(http.StatusOK, "{}"),
	)
}

func (b *bhsFixture) OnLongestTipBlockHeaderResponseWith(opts ...LongestChainTipOptions) {
	b.Helper()
	for _, o := range opts {
		o(b.longestChainTip)
	}
}

func (b *bhsFixture) OnMerkleRootVerifyResponse(height uint32, root, state string) {
	b.Helper()
	b.transport.RegisterResponder(
		http.MethodPost,
		verifyMerkleRootPath,
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]any{
			"blockHeight":       height,
			"merkleRoot":        root,
			"confirmationState": state,
		}),
	)
}

func (b *bhsFixture) WillRespondWithInternalFailure() {
	b.Helper()
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		b.bhsAnyEndpointRegexFixture,
		httpmock.NewJsonResponderOrPanic(http.StatusInternalServerError, map[string]string{
			"error": http.StatusText(http.StatusInternalServerError),
		}),
	)
}

func (b *bhsFixture) WillBeUnreachable() error {
	b.Helper()

	err := net.UnknownNetworkError("bhs - tests defined this endpoint as unreachable")
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		b.bhsAnyEndpointRegexFixture,
		httpmock.NewErrorResponder(err),
	)

	b.transport.RegisterRegexpResponder(
		http.MethodPost,
		b.bhsAnyEndpointRegexFixture,
		httpmock.NewErrorResponder(err),
	)
	return err
}

func (b *bhsFixture) IsUpAndRunning() BHSFixture {
	b.Helper()
	b.transport.RegisterResponder(
		http.MethodGet,
		tipLongestPath,
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]any{
			"header":    b.longestChainTip,
			"height":    b.longestChainTip.Height,
			"hash":      b.longestChainTip.Hash,
			"state":     "ACTIVE",
			"chainWork": 0,
		}),
	)

	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(fmt.Sprintf(`^%s.*`, headerByHeightPath)),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []*headerByHeightResponse{b.headerByHeightResponse}),
	)

	return b
}

func (b *bhsFixture) HttpClient() *resty.Client {
	b.Helper()
	client := resty.New()
	client.SetTransport(b.transport)
	return client
}

func (b *bhsFixture) Transport() *httpmock.MockTransport {
	b.Helper()
	return b.transport
}

func NewBHSFixture(t testing.TB, opts ...Option) BHSFixture {
	bhsTestURLWithoutHTTPPrefix := defs.BHSTestURL[7:]
	options := to.OptionsWithDefault(FixtureOptions{transport: httpmock.NewMockTransport()}, opts...)
	return &bhsFixture{
		TB:                         t,
		transport:                  options.transport,
		bhsAnyEndpointRegexFixture: regexp.MustCompile(fmt.Sprintf(`^http:\/\/%s\/api\/v1\/.*$`, regexp.QuoteMeta(bhsTestURLWithoutHTTPPrefix))),
		longestChainTip:            defaultLongestChainTipResponse(),
		headerByHeightResponse:     defaultHeaderByHeightResponse(),
	}
}

package testservices

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
)

type longestChainTipResponse struct {
	Height        uint   `json:"height"`
	Hash          string `json:"hash"`
	Version       uint32 `json:"version"`
	MerkleRoot    string `json:"merkleRoot"`
	Timestamp     uint64 `json:"creationTimestamp"`
	Bits          uint64 `json:"bits"`
	Nonce         uint32 `json:"nonce"`
	PreviousBlock string `json:"prevBlockHash"`
}

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
	OnLongestTipBlockHeaderResponseWith(opts ...LongestChainTipOptions)
}

type bhsFixture struct {
	testing.TB
	transport       *httpmock.MockTransport
	longestChainTip *longestChainTipResponse
}

func (b *bhsFixture) WillRespondWithEmptyLongestTipBlockHeader() {
	b.transport.RegisterResponder(
		http.MethodGet, defs.BHSTestURL+"/chain/tip/longest",
		httpmock.NewJsonResponderOrPanic(http.StatusOK, longestChainTipResponse{}),
	)
}

func (b *bhsFixture) OnLongestTipBlockHeaderResponseWith(opts ...LongestChainTipOptions) {
	for _, o := range opts {
		o(b.longestChainTip)
	}
}

func (b *bhsFixture) WillRespondWithInternalFailure() {
	b.TB.Helper()
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		bhsAnyEndpointRegexFixture,
		httpmock.NewJsonResponderOrPanic(http.StatusInternalServerError, map[string]string{
			"error": http.StatusText(http.StatusInternalServerError),
		}),
	)
}

func (b *bhsFixture) WillBeUnreachable() error {
	err := net.UnknownNetworkError("bhs - tests defined this endpoint as unreachable")
	b.TB.Helper()
	b.transport.RegisterRegexpResponder(
		http.MethodGet,
		bhsAnyEndpointRegexFixture,
		httpmock.NewErrorResponder(err),
	)
	return err
}

func (b *bhsFixture) IsUpAndRunning() BHSFixture {
	b.transport.RegisterResponder(
		http.MethodGet, defs.BHSTestURL+"/chain/tip/longest",
		httpmock.NewJsonResponderOrPanic(http.StatusOK, b.longestChainTip),
	)
	return b
}

func NewBHSFixture(t testing.TB, opts ...Option) BHSFixture {
	options := to.OptionsWithDefault(FixtureOptions{

		transport: httpmock.NewMockTransport(),
	}, opts...)

	return &bhsFixture{
		TB:              t,
		transport:       options.transport,
		longestChainTip: newDefaultLongestChainTipResponse(),
	}
}

func newDefaultLongestChainTipResponse() *longestChainTipResponse {
	return &longestChainTipResponse{
		Height:        800000,
		Hash:          "0000000000000000000a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e",
		Version:       536870912,
		MerkleRoot:    "3a4b5c6d7e8f90123456789abcdef0123456789abcdef0123456789abcdef01",
		Timestamp:     1719427200,
		Bits:          386136923,
		Nonce:         2083236893,
		PreviousBlock: "00000000000000000008e7b8c6d5f4e3d2c1b0a987654321fedcba9876543210",
	}
}

var bhsTestURLWithoutHTTPprefix = defs.BHSTestURL[7:]
var bhsAnyEndpointRegexFixture = regexp.MustCompile(fmt.Sprintf(`^http:\/\/%s(?:\/.*)?$`, bhsTestURLWithoutHTTPprefix))

package testservices

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/dto"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
)

type WhatsOnChainFixture interface {
	WillRespondWithRates(status int, content string, err error)
	WillRespondWithRawTx(status int, txID, rawTx string, err error)
	OnTipBlockHeaderWillRespondWithOneElementList() []dto.BlockHeader
	OnTipBlockHeaderWillRespondWithEmptyList() []dto.BlockHeader
	WillBeUnreachable() error
	WillRespondWithInternalFailure()
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

func (f *wocFixture) OnTipBlockHeaderWillRespondWithEmptyList() []dto.BlockHeader {
	f.TB.Helper()
	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/headers?limit=1", f.network),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []dto.BlockHeader{}),
	)
	return []dto.BlockHeader{}
}

func (f *wocFixture) OnTipBlockHeaderWillRespondWithOneElementList() []dto.BlockHeader {
	body := []dto.BlockHeader{
		{
			Hash:              "00000000000000000c5f0e00dadaf092df83f98d4dd7b5c271d4ea77840d9616",
			Height:            900769,
			Version:           603979776,
			MerkleRoot:        "d2c956bb4e4630d5e3f7d3d2188033708010b7e39fe437688a885d28617df3e9",
			Time:              1749640390,
			Nonce:             3490285233,
			Bits:              "1811a0fe",
			PreviousBlockHash: "00000000000000001018b9d31586ff13e4797fa527d1bc74d33424853940c18c",
		},
	}
	f.TB.Helper()
	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/headers?limit=1", f.network),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, body),
	)
	return body
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

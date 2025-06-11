package testservices

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
)

type WhatsOnChainFixture interface {
	WillRespondWithRates(status int, content string, err error)
	WillRespondWithRawTx(status int, txID, rawTx string, err error)
	WillRespondWithMerklePath(status int, txID string, responseBody string)
	WillRespondWithBlockHeader(status int, blockHash string, responseBody string)

	WhenQueryingMerklePath(txID string) WhatsOnChainMerklePathQueryFixture
	WhenQueryingBlockHeader(blockHash string) WhatsOnChainBlockHeaderQueryFixture

	Transport() *httpmock.MockTransport
	HttpClient() *resty.Client

	WillBeUnreachable() error
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

func (f *wocFixture) WillRespondWithRates(status int, content string, err error) {
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

	f.transport.RegisterResponder("GET", fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/exchangerate", f.network), responder(status, content))
}

func (f *wocFixture) WillRespondWithRawTx(status int, txID, rawTx string, err error) {
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

	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/hex", f.network, txID)
	f.transport.RegisterResponder("GET", url, responder(status, rawTx))
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

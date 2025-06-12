package testservices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

type WhatsOnChainFixture interface {
	WillRespondWithRates(status int, content string, err error)
	WillRespondWithTipBlockHeader(status int, err error, expectedResponse any)
	WillRespondWithRawTx(status int, txID, rawTx string, err error)
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

func (f *wocFixture) WillRespondWithTipBlockHeader(status int, err error, expectedResponse any) {
	f.TB.Helper()
	responder := func(status int, expectedResponse any) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (*http.Response, error) {
			if err != nil {
				return nil, err
			}

			bb, err := json.Marshal(expectedResponse)
			require.NoError(f.TB, err)

			res := httpmock.NewStringResponse(status, string(bb))
			res.Header.Set("Content-Type", "application/json")
			return res, nil
		}
	}

	f.transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/block/headers?limit=1", f.network),
		responder(status, expectedResponse),
	)
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

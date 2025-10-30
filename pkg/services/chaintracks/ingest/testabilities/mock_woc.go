package testabilities

import (
	"net/url"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

type MockWOC struct {
	testing.TB
	transport       *httpmock.MockTransport
	network         defs.BSVNetwork
	servicesSniffer *testutils.HTTPSniffer
}

func GivenMockWOC(t testing.TB, network defs.BSVNetwork) *MockWOC {
	transport := httpmock.NewMockTransport()
	return &MockWOC{
		TB:        t,
		transport: transport,
		network:   network,
	}
}

func (m *MockWOC) WillRespondOn(urlPath string, method string) *ResponseBuilder {
	m.Helper()
	return &ResponseBuilder{
		mockServer: m,
		url:        m.GetFullURL(urlPath),
		method:     method,
	}
}

func (m *MockWOC) GetFullURL(urlPath string) string {
	m.Helper()
	baseURL, err := whatsonchain.MakeBaseURL(m.network)
	require.NoError(m, err)

	if baseURL[len(baseURL)-1] != '/' {
		baseURL += "/"
	}

	base, err := url.Parse(baseURL)
	require.NoError(m, err)

	path, err := url.Parse(urlPath)
	require.NoError(m, err)

	full := base.ResolveReference(path)
	return full.String()
}

func (m *MockWOC) HttpClient() *resty.Client {
	m.Helper()

	m.servicesSniffer = testutils.NewHTTPSniffer(m.transport)
	client := resty.New()
	client.SetTransport(m.servicesSniffer)
	return client
}

func (m *MockWOC) ServicesSniffer() *testutils.HTTPSniffer {
	m.Helper()
	require.NotNil(m, m.servicesSniffer, "Sniffer() called without setting up services fixture")
	return m.servicesSniffer
}

type ResponseBuilder struct {
	mockServer *MockWOC
	url        string
	method     string
}

func (r *ResponseBuilder) WithJSONResponse(statusCode int, body any) *MockWOC {
	r.mockServer.Helper()
	r.mockServer.transport.RegisterResponder(r.method, r.url,
		httpmock.NewJsonResponderOrPanic(statusCode, body))
	return r.mockServer
}


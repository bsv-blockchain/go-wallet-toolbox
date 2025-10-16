package testabilities

import (
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
)

type MockServer interface {
	WillRespondOn(url string, method string) ResponseBuilder
	HttpClient() *resty.Client
}

type ResponseBuilder interface {
	WithJSONResponse(statusCode int, body any) MockServer
}

type mockServer struct{
	testing.TB
	transport                    *httpmock.MockTransport
}

func newMockServer(t testing.TB) MockServer {
	transport := httpmock.NewMockTransport()
	return &mockServer{
		TB:        t,
		transport: transport,
	}
}

func (m *mockServer) WillRespondOn(url string, method string) ResponseBuilder {
	return &responseBuilder{
		mockServer: m,
		url:        url,
		method:     method,
	}
}

func (m *mockServer) HttpClient() *resty.Client {
	client := resty.New()
	client.SetTransport(m.transport)
	return client
}

type responseBuilder struct {
	mockServer *mockServer
	url        string
	method     string
}

func (r *responseBuilder) WithJSONResponse(statusCode int, body any) MockServer {
	r.mockServer.transport.RegisterResponder(r.method, r.url,
		httpmock.NewJsonResponderOrPanic(statusCode, body))
	return r.mockServer
}

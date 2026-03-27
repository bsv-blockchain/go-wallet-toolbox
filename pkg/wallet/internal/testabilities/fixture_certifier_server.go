package testabilities

import (
	"net/http/httptest"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/slogx"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/certifier"
)

// CertifierServerBuilder is a builder interface for configuring a test certifier server.
type CertifierServerBuilder interface {
	WithCertifierWallet(wallet sdk.Interface) CertifierServerBuilder
	Started() CertifierServerFixture
}

// CertifierServerFixture represents a running test certifier server
type CertifierServerFixture interface {
	URL() string
}

type certifierServerBuilder struct {
	testing.TB

	serverWallet sdk.Interface
}

type certifierServerFixture struct {
	testing.TB

	server *httptest.Server
}

func (b *certifierServerBuilder) WithCertifierWallet(wallet sdk.Interface) CertifierServerBuilder {
	b.serverWallet = wallet
	return b
}

func (b *certifierServerBuilder) Started() CertifierServerFixture {
	b.Helper()

	if b.serverWallet == nil {
		b.Fatal("certifier wallet must be provided via WithCertifierWallet()")
	}

	logger := slogx.NewTestLogger(b.TB)

	// Create the real certifier server
	certifierServer, err := certifier.New(
		b.serverWallet,
		certifier.WithLogger(logger),
		certifier.WithOriginator("test-certifier"),
	)
	if err != nil {
		b.Fatalf("failed to create certifier server: %v", err)
	}

	// Start test server with the real certifier handler
	server := httptest.NewServer(certifierServer.Handler())

	b.Cleanup(func() {
		server.Close()
	})

	return &certifierServerFixture{
		TB:     b.TB,
		server: server,
	}
}

// URL returns the base URL of the test server
func (f *certifierServerFixture) URL() string {
	return f.server.URL
}

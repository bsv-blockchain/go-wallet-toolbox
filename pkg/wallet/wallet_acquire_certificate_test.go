package wallet_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/auth"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/util"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	certs_testabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/stretchr/testify/require"
)

func (s *WalletTestSuite) Test_AcquireCertificate() {
	t := s.T()

	s.Run("should return and store certificate in the storage based on given arguments", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		key, err := aliceWallet.GetPublicKey(t.Context(), wallet.GetPublicKeyArgs{IdentityKey: true}, fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.NotNil(t, key)

		// and:
		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)

		// then:
		actual, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		certs_testabilities.AssertWalletCertificateEquality(t, actual, args, aliceWallet)
	})

	s.Run("should fail when certifier is missing", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
		args.Certifier = nil // missing certifier

		// when:
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, cert)
	})

	s.Run("should fail when signature is missing", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
		args.Signature = nil // invalid

		// when:
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, cert)
	})

	s.Run("should fail when revocation outpoint is invalid", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
		args.RevocationOutpoint = nil // invalid

		// when:
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, cert)
	})

	s.Run("should not create a duplicate when certificate already exists", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)

		first, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.NotNil(t, first)

		// when:
		second, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, second)
	})
}

func (s *WalletTestSuite) Test_AcquireCertificate_IssuanceProtocol() {
	t := s.T()

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
	args.AcquisitionProtocol = wallet.AcquisitionProtocolIssuance

	// and:
	pubKeyHex := "02bbc996771abe50be940a9cfd91d6f28a70d139f340bedc8cdd4f236e5e9c9889"
	pubKey, _ := ec.PublicKeyFromString(pubKeyHex)
	requestID := make([]byte, 32)
	copy(requestID, "test-request-id-123456789012345") // pad to 32 bytes
	payload := encodeGeneralPayload(requestID, "POST", "/test", "", map[string]string{"Content-Type": "application/json"}, []byte(`{"foo":"bar"}`))

	// and:
	client := NewTestClient(CreateMockResponse(200, &auth.AuthMessage{
		Version:     "0.1",
		MessageType: auth.MessageTypeGeneral,
		IdentityKey: pubKey,
		Payload:     payload,
		Nonce:       "dGVzdG5vbmNlYXRlc3Rub25jZWF0ZXN0bm9uY2VhdGVzdG5vbmNlYXRlc3Rub25jZWF0ZXN0bm9uY2Vh",
		YourNonce:   "dGVzdG5vbmNlYnRlc3Rub25jZWJ0ZXN0bm9uY2VidGVzdG5vbmNlYnRlc3Rub25jZWI=",
	}))

	// and:
	aliceWallet := given.
		Wallet().
		WithActiveStorage(s.StorageType).
		WithServices().
		WithHTTPClient(client).
		ForUser(testusers.Alice)

	// when:
	actual, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

	// then:
	require.NoError(t, err)
	require.NotNil(t, actual)
}

// Helper to encode a valid general payload for the test
func encodeGeneralPayload(requestId []byte, method, path, search string, headers map[string]string, body []byte) []byte {
	w := util.NewWriter()
	w.WriteBytes(requestId)
	w.WriteString(method)
	w.WriteString(path)
	w.WriteString(search)
	w.WriteVarInt(uint64(len(headers)))
	for k, v := range headers {
		w.WriteString(k)
		w.WriteString(v)
	}
	w.WriteVarInt(uint64(len(body)))
	w.WriteBytes(body)
	return w.Buf
}

type MockRoundTripper struct {
	MockResponse *http.Response
	MockError    error
}

func CreateMockResponse(status int, data any) (*http.Response, error) {
	// 1. Marshal the custom struct into JSON bytes
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// 2. Create an io.ReadCloser (like an http.Response Body needs) from the bytes
	bodyReader := io.NopCloser(bytes.NewReader(bodyBytes))

	// 3. Create and populate the mock http.Response
	return &http.Response{
		StatusCode:    status,
		Body:          bodyReader,
		Header:        http.Header{"Content-Type": {"application/json"}},
		ContentLength: int64(len(bodyBytes)),

		// Required fields for a valid Response
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
	}, nil
}

// RoundTrip implements the http.RoundTripper interface
func (mrt *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Optionally inspect the request (req) here
	// return a canned response and error
	return mrt.MockResponse, mrt.MockError
}

func NewTestClient(res *http.Response, err error) *http.Client {
	return &http.Client{
		Transport: &MockRoundTripper{MockResponse: res, MockError: err},
	}
}

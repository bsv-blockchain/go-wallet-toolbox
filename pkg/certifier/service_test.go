package certifier

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	walletcerts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/certificates"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
)

func TestSignCertificateStashesIssuedCertificate(t *testing.T) {
	stash := newTestStash(t)
	wallet := newTestCertifierWallet(t)
	service := newTestCertificateService(t, wallet, stash)
	clientKey := newTestClientKey(t)
	req := testIssuanceRequest()

	resp, err := service.SignCertificate(t.Context(), req, clientKey)

	require.NoError(t, err)
	stashed, err := stash.FindBySerialNumber(t.Context(), resp.Certificate.SerialNumber)
	require.NoError(t, err)
	require.Equal(t, resp.Certificate.Type, stashed.Type)
	require.Equal(t, clientKey.ToDERHex(), stashed.Subject)
	require.Equal(t, resp.Certificate.Certifier, stashed.Certifier)
	require.Equal(t, resp.Certificate.RevocationOutpoint, stashed.RevocationOutpoint)
	require.Equal(t, resp.Certificate.Signature, stashed.Signature)
	require.Equal(t, wallet.revocationTx, stashed.RevocationTx)
	require.Equal(t, []IssuedCertificateField{
		{Name: "email", Value: req.Fields["email"], MasterKey: req.MasterKeyring["email"]},
		{Name: "name", Value: req.Fields["name"], MasterKey: req.MasterKeyring["name"]},
	}, stashed.Fields)

	byOutpoint, err := stash.FindByRevocationOutpoint(t.Context(), resp.Certificate.RevocationOutpoint)
	require.NoError(t, err)
	require.Equal(t, stashed.ID, byOutpoint.ID)

	bySubject, err := stash.FindBySubject(t.Context(), clientKey.ToDERHex())
	require.NoError(t, err)
	require.Len(t, bySubject, 1)
	require.Equal(t, stashed.ID, bySubject[0].ID)
}

func TestSignCertificateFailsWhenStashFails(t *testing.T) {
	stash := &failingStash{err: errors.New("stash unavailable")}
	service := newTestCertificateService(t, newTestCertifierWallet(t), stash)

	resp, err := service.SignCertificate(t.Context(), testIssuanceRequest(), newTestClientKey(t))

	require.ErrorContains(t, err, "stash unavailable")
	require.Nil(t, resp)
	require.Equal(t, 1, stash.calls)
}

func TestSignCertificateDoesNotStashWhenRevocationOutputFails(t *testing.T) {
	stash := newTestStash(t)
	wallet := newTestCertifierWallet(t)
	wallet.createActionErr = errors.New("insufficient funds")
	service := newTestCertificateService(t, wallet, stash)
	clientKey := newTestClientKey(t)

	resp, err := service.SignCertificate(t.Context(), testIssuanceRequest(), clientKey)

	require.ErrorContains(t, err, "insufficient funds")
	require.Nil(t, resp)
	certs, err := stash.FindBySubject(t.Context(), clientKey.ToDERHex())
	require.NoError(t, err)
	require.Empty(t, certs)
}

type testCertifierWallet struct {
	*sdk.CompletedProtoWallet

	revocationTxid  chainhash.Hash
	revocationTx    []byte
	createActionErr error
}

func newTestCertifierWallet(t *testing.T) *testCertifierWallet {
	t.Helper()
	key, err := ec.NewPrivateKey()
	require.NoError(t, err)
	protoWallet, err := sdk.NewCompletedProtoWallet(key)
	require.NoError(t, err)

	return &testCertifierWallet{
		CompletedProtoWallet: protoWallet,
		revocationTxid:       chainhash.DoubleHashH([]byte("revocation tx")),
		revocationTx:         []byte{0x01, 0x01, 0x01, 0x01, 0xbe, 0xef},
	}
}

func (w *testCertifierWallet) CreateAction(_ context.Context, _ sdk.CreateActionArgs, _ string) (*sdk.CreateActionResult, error) {
	if w.createActionErr != nil {
		return nil, w.createActionErr
	}
	return &sdk.CreateActionResult{Txid: w.revocationTxid, Tx: w.revocationTx}, nil
}

type failingStash struct {
	CertificateStash

	err   error
	calls int
}

func (s *failingStash) Save(_ context.Context, _ *IssuedCertificate) error {
	s.calls++
	return s.err
}

func newTestCertificateService(t *testing.T, wallet sdk.Interface, stash CertificateStash) *CertificateService {
	t.Helper()
	cfg := defaultConfig()
	cfg.Logger = logging.NewTestLogger(t)
	cfg.Randomizer = randomizer.New()
	cfg.Stash = stash
	return NewCertificateService(wallet, cfg)
}

func newTestClientKey(t *testing.T) *ec.PublicKey {
	t.Helper()
	key, err := ec.NewPrivateKey()
	require.NoError(t, err)
	return key.PubKey()
}

func testIssuanceRequest() *walletcerts.ProtocolIssuanceRequest {
	b64 := base64.StdEncoding.EncodeToString
	return &walletcerts.ProtocolIssuanceRequest{
		Type:  b64([]byte("test certificate type")),
		Nonce: b64([]byte("client nonce")),
		Fields: map[string]string{
			"name":  b64([]byte("encrypted name")),
			"email": b64([]byte("encrypted email")),
		},
		MasterKeyring: map[string]string{
			"name":  b64([]byte("name key")),
			"email": b64([]byte("email key")),
		},
	}
}

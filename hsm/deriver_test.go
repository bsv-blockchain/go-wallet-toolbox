package hsm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"math/big"
	"sync/atomic"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type equivalenceSuite struct {
	suite.Suite

	root *ec.PrivateKey
	sdk  *sdk.KeyDeriver
	hsm  *Deriver
}

func TestSoftwareBackendMatchesSDK(t *testing.T) {
	suite.Run(t, new(equivalenceSuite))
}

func (s *equivalenceSuite) SetupTest() {
	root, err := ec.NewPrivateKey()
	s.Require().NoError(err)
	backend, err := NewSoftwareBackend(root)
	s.Require().NoError(err)
	deriver, err := New(s.T().Context(), backend, WithPublicKeyCacheSize(8))
	s.Require().NoError(err)
	s.root = root
	s.sdk = sdk.NewKeyDeriver(root)
	s.hsm = deriver
}

func (s *equivalenceSuite) TearDownTest() {
	s.Require().NoError(s.hsm.Close())
	s.Require().NoError(s.hsm.Close())
}

func (s *equivalenceSuite) TestIdentity() {
	s.Require().True(s.hsm.IdentityKey().IsEqual(s.sdk.IdentityKey()))
}

func (s *equivalenceSuite) TestDerivePublicKey() {
	for _, tc := range s.cases() {
		s.Run(tc.name, func() {
			got, err := s.hsm.DerivePublicKey(s.T().Context(), tc.protocol, tc.keyID, tc.counterparty, tc.forSelf)
			s.Require().NoError(err)
			want, err := s.sdk.DerivePublicKey(tc.protocol, tc.keyID, tc.counterparty, tc.forSelf)
			s.Require().NoError(err)
			s.Require().True(got.IsEqual(want))

			again, err := s.hsm.DerivePublicKey(s.T().Context(), tc.protocol, tc.keyID, tc.counterparty, tc.forSelf)
			s.Require().NoError(err)
			s.Require().True(again.IsEqual(want))
		})
	}
}

func (s *equivalenceSuite) TestSignVerifiesAndMatchesSDK() {
	digest := sha256.Sum256([]byte("hsm software sign"))
	for _, tc := range s.cases() {
		s.Run(tc.name, func() {
			sig, err := s.hsm.Sign(s.T().Context(), tc.protocol, tc.keyID, tc.counterparty, digest[:])
			s.Require().NoError(err)
			pub, err := s.sdk.DerivePublicKey(tc.protocol, tc.keyID, tc.counterparty, true)
			s.Require().NoError(err)
			s.Require().True(sig.Verify(digest[:], pub))

			priv, err := s.sdk.DerivePrivateKey(tc.protocol, tc.keyID, tc.counterparty)
			s.Require().NoError(err)
			want, err := priv.Sign(digest[:])
			s.Require().NoError(err)
			s.Require().True(bytes.Equal(sig.Serialize(), want.Serialize()))
		})
	}
}

func (s *equivalenceSuite) TestSymmetricKeyAndHMAC() {
	data := []byte("brc-43 hmac payload")
	for _, tc := range s.cases() {
		s.Run(tc.name, func() {
			got, err := s.hsm.DeriveSymmetricKey(s.T().Context(), tc.protocol, tc.keyID, tc.counterparty)
			s.Require().NoError(err)
			want, err := s.sdk.DeriveSymmetricKey(tc.protocol, tc.keyID, tc.counterparty)
			s.Require().NoError(err)
			requireEqualSecret(s.T(), got.ToBytes(), want.ToBytes())

			mac, err := s.hsm.CreateHMAC(s.T().Context(), tc.protocol, tc.keyID, tc.counterparty, data)
			s.Require().NoError(err)
			proto, err := sdk.NewProtoWallet(sdk.ProtoWalletArgs{Type: sdk.ProtoWalletArgsTypeKeyDeriver, KeyDeriver: s.sdk})
			s.Require().NoError(err)
			wantMAC, err := proto.CreateHMAC(s.T().Context(), sdk.CreateHMACArgs{
				EncryptionArgs: sdk.EncryptionArgs{ProtocolID: tc.protocol, KeyID: tc.keyID, Counterparty: tc.counterparty},
				Data:           data,
			}, "")
			s.Require().NoError(err)
			requireEqualSecret(s.T(), mac[:], wantMAC.HMAC[:])
		})
	}
}

func (s *equivalenceSuite) TestEncryptRoundTrip() {
	tc := s.cases()[0]
	plaintext := []byte("toolbox hsm encrypt")
	ciphertext, err := s.hsm.Encrypt(s.T().Context(), tc.protocol, tc.keyID, tc.counterparty, plaintext)
	s.Require().NoError(err)
	key, err := s.sdk.DeriveSymmetricKey(tc.protocol, tc.keyID, tc.counterparty)
	s.Require().NoError(err)
	opened, err := key.Decrypt(ciphertext)
	s.Require().NoError(err)
	s.Require().Equal(plaintext, opened)

	sdkCipher, err := key.Encrypt(plaintext)
	s.Require().NoError(err)
	got, err := s.hsm.Decrypt(s.T().Context(), tc.protocol, tc.keyID, tc.counterparty, sdkCipher)
	s.Require().NoError(err)
	s.Require().Equal(plaintext, got)
}

func (s *equivalenceSuite) TestRevealSpecificSecret() {
	for _, tc := range s.cases() {
		s.Run(tc.name, func() {
			got, err := s.hsm.RevealSpecificSecret(s.T().Context(), tc.counterparty, tc.protocol, tc.keyID)
			s.Require().NoError(err)
			defer zero(got)
			want, err := s.sdk.RevealSpecificSecret(tc.counterparty, tc.protocol, tc.keyID)
			s.Require().NoError(err)
			defer zero(want)
			requireEqualSecret(s.T(), got, want)
		})
	}
}

func (s *equivalenceSuite) TestRevealCounterpartySecret() {
	other, err := ec.NewPrivateKey()
	s.Require().NoError(err)
	cp := sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: other.PubKey()}
	got, err := s.hsm.RevealCounterpartySecret(s.T().Context(), cp)
	s.Require().NoError(err)
	want, err := s.sdk.RevealCounterpartySecret(cp)
	s.Require().NoError(err)
	s.Require().True(got.IsEqual(want))

	_, err = s.hsm.RevealCounterpartySecret(s.T().Context(), sdk.Counterparty{Type: sdk.CounterpartyTypeSelf})
	s.Require().Error(err)
}

func (s *equivalenceSuite) TestRejectsBadInvoice() {
	_, err := s.hsm.DerivePublicKey(s.T().Context(), sdk.Protocol{SecurityLevel: 0, Protocol: "bad"}, "k", sdk.Counterparty{Type: sdk.CounterpartyTypeSelf}, true)
	s.Require().Error(err)
	_, err = s.hsm.DerivePublicKey(s.T().Context(), sdk.Protocol{SecurityLevel: 9, Protocol: "wallet"}, "k", sdk.Counterparty{Type: sdk.CounterpartyTypeSelf}, true)
	s.Require().Error(err)
}

func (s *equivalenceSuite) TestHighSNormalises() {
	priv, err := ec.NewPrivateKey()
	s.Require().NoError(err)
	digest := sha256.Sum256([]byte("low-s"))
	sig, err := priv.Sign(digest[:])
	s.Require().NoError(err)
	n := ec.S256().Params().N
	raw := make([]byte, 64)
	rb := sig.R.Bytes()
	highS := new(big.Int).Sub(n, sig.S)
	sb := highS.Bytes()
	copy(raw[32-len(rb):32], rb)
	copy(raw[64-len(sb):], sb)
	parsed, err := signatureFromRS(raw)
	s.Require().NoError(err)
	s.Require().True(parsed.Verify(digest[:], priv.PubKey()))
	s.Require().True(bytes.Equal(parsed.Serialize(), sig.Serialize()))
}

func (s *equivalenceSuite) cases() []derivationCase {
	other, err := ec.NewPrivateKey()
	s.Require().NoError(err)
	protocol := sdk.Protocol{SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty, Protocol: "3241645161d8"}
	otherProtocol := sdk.Protocol{SecurityLevel: sdk.SecurityLevelEveryApp, Protocol: "wallet toolbox"}
	return []derivationCase{
		{name: "self", protocol: protocol, keyID: "prefix suffix", counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeSelf}, forSelf: true},
		{name: "self public", protocol: protocol, keyID: "prefix suffix", counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeSelf}, forSelf: false},
		{name: "other for self", protocol: protocol, keyID: "invoice 1", counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: other.PubKey()}, forSelf: true},
		{name: "other for counterparty", protocol: otherProtocol, keyID: "second key", counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: other.PubKey()}, forSelf: false},
		{name: "anyone", protocol: otherProtocol, keyID: "anyone key", counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeAnyone}, forSelf: true},
	}
}

type derivationCase struct {
	name         string
	protocol     sdk.Protocol
	keyID        string
	counterparty sdk.Counterparty
	forSelf      bool
}

func TestPublicKeyCacheSkipsRepeatHMAC(t *testing.T) {
	root, err := ec.NewPrivateKey()
	require.NoError(t, err)
	inner, err := NewSoftwareBackend(root)
	require.NoError(t, err)
	counting := &countingBackend{Backend: inner}
	deriver, err := New(t.Context(), counting, WithPublicKeyCacheSize(4))
	require.NoError(t, err)
	defer func() { require.NoError(t, deriver.Close()) }()

	protocol := sdk.Protocol{SecurityLevel: sdk.SecurityLevelEveryApp, Protocol: "cache test"}
	cp := sdk.Counterparty{Type: sdk.CounterpartyTypeSelf}
	_, err = deriver.DerivePublicKey(t.Context(), protocol, "same", cp, true)
	require.NoError(t, err)
	_, err = deriver.DerivePublicKey(t.Context(), protocol, "same", cp, true)
	require.NoError(t, err)
	require.Equal(t, int32(1), counting.n.Load())
}

func TestNewRejectsNilBackend(t *testing.T) {
	_, err := New(t.Context(), nil)
	require.Error(t, err)
	_, err = NewSoftwareBackend(nil)
	require.Error(t, err)
}

type countingBackend struct {
	Backend

	n atomic.Int32
}

func (c *countingBackend) OpenHMAC(ctx context.Context, peer, message []byte, reveal bool) ([]byte, []byte, HMACSession, error) {
	c.n.Add(1)
	return c.Backend.OpenHMAC(ctx, peer, message, reveal)
}

// requireEqualSecret fails without printing the bytes.
func requireEqualSecret(t *testing.T, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("secret mismatch (len got=%d want=%d)", len(got), len(want))
	}
}

func TestSignWithPublicKeyUsesOneDerivation(t *testing.T) {
	root, err := ec.NewPrivateKey()
	require.NoError(t, err)
	soft, err := NewSoftwareBackend(root)
	require.NoError(t, err)
	backend := &countingBackend{Backend: soft}
	d, err := New(t.Context(), backend)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.Close()) })
	protocol := sdk.Protocol{SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty, Protocol: "3241645161d8"}
	cp := sdk.Counterparty{Type: sdk.CounterpartyTypeAnyone}
	digest := sha256.Sum256([]byte("one derivation per input"))
	sig, pub, err := d.SignWithPublicKey(t.Context(), protocol, "prefix suffix", cp, digest[:])
	require.NoError(t, err)
	require.True(t, sig.Verify(digest[:], pub))
	expected, err := sdk.NewKeyDeriver(root).DerivePublicKey(protocol, "prefix suffix", cp, true)
	require.NoError(t, err)
	require.True(t, pub.IsEqual(expected))
	cached, err := d.DerivePublicKey(t.Context(), protocol, "prefix suffix", cp, true)
	require.NoError(t, err)
	require.True(t, pub.IsEqual(cached))
	require.EqualValues(t, 1, backend.n.Load())
}

type observedBackend struct {
	Backend
	h       []byte
	point   []byte
	fail    bool
	session *observedSession
}
type observedSession struct {
	HMACSession
	closed bool
}

func (s *observedSession) Close() error { s.closed = true; return s.HMACSession.Close() }
func (b *observedBackend) OpenHMAC(ctx context.Context, peer, message []byte, reveal bool) ([]byte, []byte, HMACSession, error) {
	h, p, s, err := b.Backend.OpenHMAC(ctx, peer, message, reveal)
	b.h, b.point = h, p
	if s != nil {
		b.session = &observedSession{HMACSession: s}
		s = b.session
	}
	if err == nil && b.fail {
		err = context.Canceled
	}
	return h, p, s, err
}
func TestTemporaryDerivationMaterialIsWiped(t *testing.T) {
	for _, fail := range []bool{false, true} {
		for _, reveal := range []bool{false, true} {
			root, err := ec.NewPrivateKey()
			require.NoError(t, err)
			soft, err := NewSoftwareBackend(root)
			require.NoError(t, err)
			b := &observedBackend{Backend: soft, fail: fail}
			d, err := New(t.Context(), b)
			require.NoError(t, err)
			cp := sdk.Counterparty{Type: sdk.CounterpartyTypeAnyone}
			if reveal {
				_, err = d.RevealCounterpartySecret(t.Context(), cp)
			} else {
				_, _, err = d.SignWithPublicKey(t.Context(), sdk.Protocol{SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty, Protocol: "3241645161d8"}, "wipe", cp, make([]byte, 32))
			}
			if fail {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.NoError(t, err)
			}
			require.Len(t, b.h, 32)
			requireEqualSecret(t, b.h, make([]byte, len(b.h)))
			requireEqualSecret(t, b.point, make([]byte, len(b.point)))
			require.True(t, b.session.closed)
			require.NoError(t, d.Close())
		}
	}
}

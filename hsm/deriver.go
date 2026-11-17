package hsm

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sync"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

// DefaultPublicKeyCacheSize is the number of derived public keys kept when
// New is called without WithPublicKeyCacheSize.
const DefaultPublicKeyCacheSize = 1024

// Option configures a Deriver.
type Option func(*options)

type options struct {
	cacheSize int
}

// WithPublicKeyCacheSize sets how many derived public keys are cached.
// Zero disables the cache. Public keys are not secret; scalars are not cached.
func WithPublicKeyCacheSize(n int) Option {
	return func(o *options) {
		o.cacheSize = n
	}
}

// Deriver implements BRC-42 and BRC-43 with Backend.
// The root private key never crosses the Backend boundary.
type Deriver struct {
	backend  Backend
	identity *ec.PublicKey

	mu    sync.Mutex
	cache *pubCache

	closeOnce sync.Once
	closeErr  error
}

// New binds backend and reads the root public key.
// ctx bounds that read. backend is closed by Close.
func New(ctx context.Context, backend Backend, opts ...Option) (*Deriver, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if backend == nil {
		return nil, fmt.Errorf("hsm backend is required")
	}
	cfg := options{cacheSize: DefaultPublicKeyCacheSize}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.cacheSize < 0 {
		return nil, fmt.Errorf("public key cache size must be >= 0")
	}
	identity, err := backend.Identity(ctx)
	if err != nil {
		return nil, fmt.Errorf("read root public key: %w", err)
	}
	if identity == nil || !identity.Validate() {
		return nil, fmt.Errorf("root public key is not on the curve")
	}
	return &Deriver{
		backend:  backend,
		identity: identity,
		cache:    newPubCache(cfg.cacheSize),
	}, nil
}

// IdentityKey returns the root public key.
func (d *Deriver) IdentityKey() *ec.PublicKey {
	if d == nil {
		return nil
	}
	return d.identity
}

// DerivePublicKey returns the BRC-42 child public key.
// forSelf selects root + h*G. Otherwise the counterparty point plus h*G
// is returned. h is wiped before this method returns.
func (d *Deriver) DerivePublicKey(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, forSelf bool) (*ec.PublicKey, error) {
	peer, invoice, err := d.factorInputs(ctx, protocol, keyID, counterparty)
	if err != nil {
		return nil, err
	}
	key := pubCacheKey{
		level:   int(protocol.SecurityLevel),
		proto:   protocol.Protocol,
		keyID:   keyID,
		peer:    string(peer.Compressed()),
		forSelf: forSelf,
	}
	d.mu.Lock()
	if pub, ok := d.cache.get(key); ok {
		d.mu.Unlock()
		return pub, nil
	}
	d.mu.Unlock()

	h, _, session, err := d.backend.OpenHMAC(ctx, peer.Compressed(), []byte(invoice), false)
	if session != nil {
		defer func() { _ = session.Close() }()
	}
	defer zero(h)
	if err != nil {
		return nil, fmt.Errorf("derive public key: %w", err)
	}

	base := peer
	if forSelf {
		base = d.identity
	}
	pub, err := addScalarG(base, h)
	if err != nil {
		return nil, fmt.Errorf("derive public key: %w", err)
	}
	d.mu.Lock()
	d.cache.put(key, pub)
	d.mu.Unlock()
	return pub, nil
}

// DeriveSymmetricKey returns the BRC-43 symmetric key (the X coordinate of
// the product of the two child points). The coordinate is not the root
// scalar; the counterparty can recompute it, which is what makes the key
// useful. It is still wiped from temporary buffers.
func (d *Deriver) DeriveSymmetricKey(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty) (*ec.SymmetricKey, error) {
	peer, invoice, err := d.factorInputs(ctx, protocol, keyID, counterparty)
	if err != nil {
		return nil, err
	}
	h, _, session, err := d.backend.OpenHMAC(ctx, peer.Compressed(), []byte(invoice), false)
	if session != nil {
		defer func() { _ = session.Close() }()
	}
	defer zero(h)
	if err != nil {
		return nil, fmt.Errorf("derive symmetric key: %w", err)
	}

	// BRC-43: X( (root+h) * (counterparty + h*G) ).
	second, err := addScalarG(peer, h)
	if err != nil {
		return nil, fmt.Errorf("derive symmetric key: %w", err)
	}
	x, err := session.ChildSharedX(second.Compressed())
	if err != nil {
		return nil, fmt.Errorf("derive symmetric key: %w", err)
	}
	if len(x) == 0 {
		return nil, fmt.Errorf("derive symmetric key: empty X coordinate")
	}
	return ec.NewSymmetricKey(x), nil
}

// Sign signs digest with the BRC-42 child private key inside Backend.
// The returned signature is low-S DER.
func (d *Deriver) Sign(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, digest []byte) (*ec.Signature, error) {
	sig, _, err := d.SignWithPublicKey(ctx, protocol, keyID, counterparty, digest)
	return sig, err
}

// SignWithPublicKey signs and derives the matching child public key using one
// ECDH/HMAC session. BRC-29 unlockers can avoid a separate token round trip.
// Only the public key is cached; the derivation scalar is wiped on every exit.
func (d *Deriver) SignWithPublicKey(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, digest []byte) (*ec.Signature, *ec.PublicKey, error) {
	if len(digest) == 0 {
		return nil, nil, fmt.Errorf("digest is empty")
	}
	if new(big.Int).SetBytes(digest).BitLen() > ec.S256().Params().N.BitLen() {
		return nil, nil, fmt.Errorf("digest is too large to sign")
	}
	peer, invoice, err := d.factorInputs(ctx, protocol, keyID, counterparty)
	if err != nil {
		return nil, nil, err
	}
	h, _, session, err := d.backend.OpenHMAC(ctx, peer.Compressed(), []byte(invoice), false)
	if session != nil {
		defer func() { _ = session.Close() }()
	}
	defer zero(h)
	if err != nil {
		return nil, nil, fmt.Errorf("sign: %w", err)
	}
	pub, err := addScalarG(d.identity, h)
	if err != nil {
		return nil, nil, fmt.Errorf("sign public key: %w", err)
	}
	raw, err := session.ChildSign(digest)
	if err != nil {
		return nil, nil, fmt.Errorf("sign: %w", err)
	}
	defer zero(raw)
	sig, err := signatureFromRS(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("sign: %w", err)
	}
	d.mu.Lock()
	d.cache.put(pubCacheKey{level: int(protocol.SecurityLevel), proto: protocol.Protocol, keyID: keyID, peer: string(peer.Compressed()), forSelf: true}, pub)
	d.mu.Unlock()
	return sig, pub, nil
}

// CreateHMAC returns HMAC-SHA256 of data under the BRC-43 symmetric key,
// using the same minimal-length key encoding as go-sdk ProtoWallet.
func (d *Deriver) CreateHMAC(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, data []byte) ([32]byte, error) {
	var out [32]byte
	key, err := d.DeriveSymmetricKey(ctx, protocol, keyID, counterparty)
	if err != nil {
		return out, err
	}
	sum := hmacSHA256(minimalHMACKey(key.ToBytes()), data)
	if len(sum) != len(out) {
		return out, fmt.Errorf("hmac length %d", len(sum))
	}
	copy(out[:], sum)
	zero(sum)
	return out, nil
}

// Encrypt encrypts plaintext with the BRC-43 symmetric key (AES-GCM, the
// go-sdk encoding, including a random IV).
func (d *Deriver) Encrypt(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, plaintext []byte) ([]byte, error) {
	key, err := d.DeriveSymmetricKey(ctx, protocol, keyID, counterparty)
	if err != nil {
		return nil, err
	}
	ciphertext, err := key.Encrypt(plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	return ciphertext, nil
}

// Decrypt decrypts ciphertext produced by Encrypt or by go-sdk under the
// same derivation inputs.
func (d *Deriver) Decrypt(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, ciphertext []byte) ([]byte, error) {
	key, err := d.DeriveSymmetricKey(ctx, protocol, keyID, counterparty)
	if err != nil {
		return nil, err
	}
	plaintext, err := key.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

// RevealSpecificSecret returns the BRC-42 scalar h for this counterparty,
// protocol, and key ID. h is the linkage value BRC-69 reveals; returning it
// is the operation, not a leak of the root key.
func (d *Deriver) RevealSpecificSecret(ctx context.Context, counterparty sdk.Counterparty, protocol sdk.Protocol, keyID string) ([]byte, error) {
	peer, invoice, err := d.factorInputs(ctx, protocol, keyID, counterparty)
	if err != nil {
		return nil, err
	}
	h, _, session, err := d.backend.OpenHMAC(ctx, peer.Compressed(), []byte(invoice), false)
	if session != nil {
		defer func() { _ = session.Close() }()
	}
	if err != nil {
		return nil, fmt.Errorf("reveal specific secret: %w", err)
	}
	return h, nil
}

// RevealCounterpartySecret returns the ECDH shared point with counterparty.
// The point is extracted from the token (the BRC-69 linkage). Self is
// rejected, matching go-sdk.
func (d *Deriver) RevealCounterpartySecret(ctx context.Context, counterparty sdk.Counterparty) (*ec.PublicKey, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if counterparty.Type == sdk.CounterpartyTypeSelf {
		return nil, fmt.Errorf("counterparty secrets cannot be revealed for counterparty=self")
	}
	peer, err := d.normalize(counterparty)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(peer.Compressed(), d.identity.Compressed()) {
		return nil, fmt.Errorf("counterparty secrets cannot be revealed if counterparty key is self")
	}
	// The shared point does not depend on the HMAC message. A one-byte
	// message still exercises the HMAC step the token requires before the
	// secret object can be released.
	h, point, session, err := d.backend.OpenHMAC(ctx, peer.Compressed(), []byte{0}, true)
	if session != nil {
		defer func() { _ = session.Close() }()
	}
	defer zero(h)
	defer zero(point)
	if err != nil {
		return nil, fmt.Errorf("reveal counterparty secret: %w", err)
	}
	pub, err := ec.ParsePubKey(point)
	if err != nil {
		return nil, fmt.Errorf("reveal counterparty secret: %w", err)
	}
	return pub, nil
}

// Close releases the backend. It is safe to call more than once.
func (d *Deriver) Close() error {
	if d == nil {
		return nil
	}
	d.closeOnce.Do(func() {
		d.closeErr = d.backend.Close()
	})
	return d.closeErr
}

func (d *Deriver) factorInputs(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty) (*ec.PublicKey, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	peer, err := d.normalize(counterparty)
	if err != nil {
		return nil, "", err
	}
	invoice, err := computeInvoiceNumber(protocol, keyID)
	if err != nil {
		return nil, "", err
	}
	return peer, invoice, nil
}

func (d *Deriver) normalize(counterparty sdk.Counterparty) (*ec.PublicKey, error) {
	switch counterparty.Type {
	case sdk.CounterpartyTypeSelf:
		return d.identity, nil
	case sdk.CounterpartyTypeOther:
		if counterparty.Counterparty == nil {
			return nil, fmt.Errorf("counterparty public key required for other")
		}
		if !counterparty.Counterparty.Validate() {
			return nil, fmt.Errorf("counterparty public key is not on the curve")
		}
		return counterparty.Counterparty, nil
	case sdk.CounterpartyTypeAnyone:
		_, pub := sdk.AnyoneKey()
		return pub, nil
	case sdk.CounterpartyUninitialized:
		return nil, fmt.Errorf("invalid counterparty, must be self, other, or anyone")
	default:
		return nil, fmt.Errorf("invalid counterparty, must be self, other, or anyone")
	}
}

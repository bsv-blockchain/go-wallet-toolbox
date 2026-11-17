package hsm

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"math/big"
	"sync"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
)

// NewSoftwareBackend returns a Backend that performs BRC-42 with root.
// root is the test double for the token key: production calls use the
// PKCS#11 backend, which never accepts a private key.
//
// The caller keeps ownership of root. Close does not wipe it.
func NewSoftwareBackend(root *ec.PrivateKey) (Backend, error) {
	if root == nil || root.D == nil {
		return nil, fmt.Errorf("software backend requires a root private key")
	}
	return &softwareBackend{root: root}, nil
}

type softwareBackend struct {
	mu     sync.Mutex
	root   *ec.PrivateKey
	closed bool
}

func (b *softwareBackend) Identity(context.Context) (*ec.PublicKey, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.root == nil {
		return nil, fmt.Errorf("software backend is closed")
	}
	return b.root.PubKey(), nil
}

func (b *softwareBackend) OpenHMAC(ctx context.Context, peerCompressed, message []byte, revealPoint bool) ([]byte, []byte, HMACSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	peer, err := ec.ParsePubKey(peerCompressed)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse counterparty public key: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.root == nil {
		return nil, nil, nil, fmt.Errorf("software backend is closed")
	}

	shared, err := b.root.DeriveSharedSecret(peer)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("software ecdh: %w", err)
	}
	point := shared.Compressed()
	mac := hmac.New(sha256.New, point)
	mac.Write(message)
	sum := mac.Sum(nil)

	out := make([]byte, len(sum))
	copy(out, sum)
	scalar := make([]byte, len(sum))
	copy(scalar, sum)

	var revealed []byte
	if revealPoint {
		revealed = make([]byte, len(point))
		copy(revealed, point)
	}
	return out, revealed, &softwareSession{
		root:   b.root,
		scalar: scalar,
	}, nil
}

func (b *softwareBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.root = nil
	return nil
}

type softwareSession struct {
	root   *ec.PrivateKey
	scalar []byte
	closed bool
}

func (s *softwareSession) ChildSign(digest []byte) ([]byte, error) {
	child, err := s.child()
	if err != nil {
		return nil, err
	}
	sig, err := child.Sign(digest)
	if err != nil {
		return nil, fmt.Errorf("software ecdsa: %w", err)
	}
	raw := make([]byte, 64)
	rb := sig.R.Bytes()
	sb := sig.S.Bytes()
	if len(rb) > 32 || len(sb) > 32 {
		return nil, fmt.Errorf("software signature scalar is wider than 32 bytes")
	}
	copy(raw[32-len(rb):32], rb)
	copy(raw[64-len(sb):], sb)
	return raw, nil
}

func (s *softwareSession) ChildSharedX(peerCompressed []byte) ([]byte, error) {
	child, err := s.child()
	if err != nil {
		return nil, err
	}
	other, err := ec.ParsePubKey(peerCompressed)
	if err != nil {
		return nil, fmt.Errorf("parse ecdh peer: %w", err)
	}
	shared, err := child.DeriveSharedSecret(other)
	if err != nil {
		return nil, fmt.Errorf("software child ecdh: %w", err)
	}
	return shared.X.Bytes(), nil
}

func (s *softwareSession) child() (*ec.PrivateKey, error) {
	if s == nil || s.closed || s.root == nil || len(s.scalar) == 0 {
		return nil, fmt.Errorf("software hmac session is closed")
	}
	// child = (root + h) mod n, using the scalar OpenHMAC returned rather
	// than deriving it a second time. A wrong h then fails the go-sdk
	// signature comparison.
	d := new(big.Int).Set(s.root.D)
	d.Add(d, new(big.Int).SetBytes(s.scalar))
	d.Mod(d, ec.S256().Params().N)
	child, _ := ec.PrivateKeyFromBytes(d.Bytes())
	if child == nil || child.D == nil || child.D.Sign() == 0 {
		return nil, fmt.Errorf("software child scalar is zero")
	}
	return child, nil
}

func (s *softwareSession) Close() error {
	if s == nil {
		return nil
	}
	zero(s.scalar)
	s.scalar = nil
	s.root = nil
	s.closed = true
	return nil
}

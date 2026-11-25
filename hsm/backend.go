package hsm

import (
	"context"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
)

// Backend is the token operation surface used by Deriver.
//
// OpenHMAC, ChildSign, and ChildSharedX run on one PKCS#11 session: session
// objects created by CKM_ECDH1_DERIVE_COMPRESSED_POINT and
// CKM_EC_SCALAR_CHILD_DERIVE are only visible in the session that created
// them. Implementations must not return the root private key.
//
// OpenHMAC returns h, the HMAC-SHA256 of the compressed shared point over
// the invoice. h is the BRC-42 scalar and it leaves the token so the caller
// can add h*G and so CKM_EC_SCALAR_CHILD_DERIVE can be given pScalar. That
// round trip is a known gap; a later Securosys mechanism could keep h as a
// handle. Callers must wipe h when they are done with it. The shared point
// itself stays inside the token unless revealPoint is set (BRC-69).
type Backend interface {
	// Identity returns the root public key. The root private key stays inside
	// the backend.
	Identity(ctx context.Context) (*ec.PublicKey, error)

	// OpenHMAC performs ECDH against peerCompressed (33-byte SEC1 point) and
	// HMAC-SHA256 over message. When revealPoint is true the compressed
	// shared point is returned; otherwise point is nil.
	// The caller must Close the session, including when ChildSign and
	// ChildSharedX are not used.
	OpenHMAC(ctx context.Context, peerCompressed, message []byte, revealPoint bool) (h, point []byte, session HMACSession, err error)

	// Close releases the token. It is safe to call more than once.
	Close() error
}

// HMACSession is the session that produced h. ChildSign and ChildSharedX
// consume that scalar without the caller passing the root key.
type HMACSession interface {
	// ChildSign derives the BRC-42 child (root scalar + h) and signs digest
	// with ECDSA. digest is the hash that CKM_ECDSA signs; it is not hashed
	// again. The signature is raw r||s (64 bytes), not DER.
	ChildSign(digest []byte) ([]byte, error)

	// ChildSharedX derives the same child and returns the minimal big-endian
	// X coordinate of ECDH(child, peerCompressed). peerCompressed is the
	// 33-byte SEC1 encoding of the other point in the BRC-43 product.
	ChildSharedX(peerCompressed []byte) ([]byte, error)

	// Close releases the session and wipes the scalar it holds.
	Close() error
}

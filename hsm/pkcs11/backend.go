//go:build pkcs11

package pkcs11

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/miekg/pkcs11"

	"github.com/bsv-blockchain/go-wallet-toolbox/hsm"
)

const (
	// Vendor mechanisms validated on Securosys Grimsel, firmware RX-3.4.0-T,
	// PKCS#11 provider 2.8.2. CKM_EC_SCALAR_CHILD_DERIVE only succeeds through
	// C_DeriveKeyPair; C_DeriveKey returns CKR_MECHANISM_INVALID.
	ckmECScalarChildDerive        = 0x80000E11
	ckmECDH1DeriveCompressedPoint = 0x80000E12

	sharedPointLen     = 33
	defaultSessionPool = 4
	maxSessionPool     = 32
)

// Config selects the token key Deriver should use.
// The PIN is read from PINFile. It is not a field on Config, and it is not logged.
// The Securosys provider reads SECUROSYS_PKCS11_CONF and SECUROSYS_SECRETS_CONF itself.
type Config struct {
	// LibraryPath is the provider shared library (P11_LIB in the Grimsel PoC).
	LibraryPath string
	// Slot is the PKCS#11 slot id. -1 selects the first slot that has a token.
	Slot int
	// PINFile is the path of a file containing the user PIN.
	PINFile string
	// KeyLabel is the CKA_LABEL of the secp256k1 token key pair.
	// The private key must be SENSITIVE, not EXTRACTABLE, and allow DERIVE and SIGN.
	KeyLabel string
	// SessionPoolSize is how many logged-in sessions to keep. Zero uses the default.
	SessionPoolSize int
	// Logf receives operational messages that contain no key material and no PIN.
	// Nil discards them.
	Logf func(string)
}

// Open logs in and finds the root key. ctx is checked before the provider
// calls; those calls themselves cannot be cancelled. The returned backend
// is closed by hsm.Deriver.Close.
func Open(ctx context.Context, cfg Config) (hsm.Backend, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg.LibraryPath == "" {
		return nil, fmt.Errorf("pkcs11 library path is required")
	}
	if cfg.PINFile == "" {
		return nil, fmt.Errorf("pkcs11 pin file is required")
	}
	if cfg.KeyLabel == "" {
		return nil, fmt.Errorf("pkcs11 key label is required")
	}
	poolSize := cfg.SessionPoolSize
	if poolSize == 0 {
		poolSize = defaultSessionPool
	}
	if poolSize < 1 || poolSize > maxSessionPool {
		return nil, fmt.Errorf("session pool size must be between 1 and %d", maxSessionPool)
	}
	pin, err := readPIN(cfg.PINFile)
	if err != nil {
		return nil, err
	}
	defer zero(pin)

	module := pkcs11.New(cfg.LibraryPath)
	if module == nil {
		return nil, fmt.Errorf("failed to load pkcs11 library")
	}
	if err = module.Initialize(); err != nil {
		module.Destroy()
		return nil, fmt.Errorf("pkcs11 initialize: %w", err)
	}
	b := &backend{
		module: module,
		lib:    cfg.LibraryPath,
		label:  cfg.KeyLabel,
		logf:   cfg.Logf,
	}
	if err = ctx.Err(); err != nil {
		_ = b.Close()
		return nil, err
	}
	if err = b.openPool(ctx, cfg.Slot, poolSize, pin); err != nil {
		_ = b.Close()
		return nil, err
	}
	return b, nil
}

type backend struct {
	module   *pkcs11.Ctx
	lib      string
	label    string
	logf     func(string)
	identity *ec.PublicKey
	pool     *pool

	once sync.Once
	err  error
}

func (b *backend) Identity(context.Context) (*ec.PublicKey, error) {
	if b.identity == nil {
		return nil, fmt.Errorf("root public key is not loaded")
	}
	return b.identity, nil
}

func (b *backend) OpenHMAC(ctx context.Context, peerCompressed, message []byte, revealPoint bool) ([]byte, []byte, hsm.HMACSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	if len(peerCompressed) != sharedPointLen {
		return nil, nil, nil, fmt.Errorf("counterparty public key length %d, want %d", len(peerCompressed), sharedPointLen)
	}
	s, err := b.pool.acquire(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	held := true
	defer func() {
		if held {
			b.pool.release(s)
		}
	}()

	secret, err := b.ecdh(s, s.root, peerCompressed, revealPoint)
	if err != nil {
		return nil, nil, nil, err
	}
	defer b.destroy(s.handle, secret)

	var point []byte
	if revealPoint {
		point, err = b.readValue(s.handle, secret)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read shared point: %w", err)
		}
	}
	if err = b.module.SignInit(s.handle, []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_SHA256_HMAC, nil)}, secret); err != nil {
		zero(point)
		return nil, nil, nil, fmt.Errorf("hmac init: %w", err)
	}
	mac, err := b.module.Sign(s.handle, message)
	if err != nil {
		zero(point)
		return nil, nil, nil, fmt.Errorf("hmac: %w", err)
	}
	if len(mac) != 32 {
		zero(mac)
		zero(point)
		return nil, nil, nil, fmt.Errorf("hmac length %d", len(mac))
	}
	scalar := make([]byte, len(mac))
	copy(scalar, mac)
	held = false
	return mac, point, &hmacSession{b: b, s: s, scalar: scalar}, nil
}

func (b *backend) Close() error {
	b.once.Do(func() {
		if b.pool != nil {
			b.pool.shutdown()
		}
		if b.module != nil {
			if err := b.module.Finalize(); err != nil {
				b.err = fmt.Errorf("pkcs11 finalize: %w", err)
			}
			b.module.Destroy()
		}
	})
	return b.err
}

func (b *backend) openPool(ctx context.Context, slotID int, n int, pin []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	slots, err := b.module.GetSlotList(true)
	if err != nil {
		return fmt.Errorf("pkcs11 slot list: %w", err)
	}
	if len(slots) == 0 {
		return fmt.Errorf("pkcs11: no slot has a token")
	}
	slot := slots[0]
	if slotID >= 0 {
		found := false
		for _, candidate := range slots {
			if candidate == uint(slotID) {
				slot = candidate
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("pkcs11 slot %d has no token", slotID)
		}
	}
	info, err := b.module.GetInfo()
	if err != nil {
		return fmt.Errorf("pkcs11 info: %w", err)
	}
	token, err := b.module.GetTokenInfo(slot)
	if err != nil {
		return fmt.Errorf("pkcs11 token info: %w", err)
	}
	b.log(fmt.Sprintf("pkcs11 library %s slot %d token %s key %s sessions %d",
		strings.TrimSpace(info.LibraryDescription), slot, strings.TrimSpace(token.Label), b.label, n))

	p := newPool(b.module, n)
	for i := 0; i < n; i++ {
		if err := ctx.Err(); err != nil {
			p.shutdown()
			return err
		}
		sh, err := b.module.OpenSession(slot, pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
		if err != nil {
			p.shutdown()
			return fmt.Errorf("pkcs11 open session: %w", err)
		}
		if err = b.module.Login(sh, pkcs11.CKU_USER, string(pin)); err != nil && !alreadyLoggedIn(err) {
			_ = b.module.CloseSession(sh)
			p.shutdown()
			return fmt.Errorf("pkcs11 login: %w", err)
		}
		root, err := findOne(b.module, sh, pkcs11.CKO_PRIVATE_KEY, b.label)
		if err != nil {
			_ = b.module.Logout(sh)
			_ = b.module.CloseSession(sh)
			p.shutdown()
			return err
		}
		if i == 0 {
			pubHandle, err := findOne(b.module, sh, pkcs11.CKO_PUBLIC_KEY, b.label)
			if err != nil {
				_ = b.module.Logout(sh)
				_ = b.module.CloseSession(sh)
				p.shutdown()
				return err
			}
			identity, err := b.readPublicKey(sh, pubHandle)
			if err != nil {
				_ = b.module.Logout(sh)
				_ = b.module.CloseSession(sh)
				p.shutdown()
				return err
			}
			b.identity = identity
		}
		p.add(&sess{pool: p, module: b.module, handle: sh, root: root})
	}
	b.pool = p
	return nil
}

func (b *backend) ecdh(s *sess, base pkcs11.ObjectHandle, peer []byte, extractable bool) (pkcs11.ObjectHandle, error) {
	params := pkcs11.NewECDH1DeriveParams(pkcs11.CKD_NULL, nil, peer)
	secret, err := b.module.DeriveKey(s.handle, []*pkcs11.Mechanism{
		pkcs11.NewMechanism(ckmECDH1DeriveCompressedPoint, params),
	}, base, secretTemplate(extractable))
	if err != nil {
		return 0, fmt.Errorf("ecdh: %w", err)
	}
	return secret, nil
}

func (b *backend) readValue(sh pkcs11.SessionHandle, obj pkcs11.ObjectHandle) ([]byte, error) {
	attrs, err := b.module.GetAttributeValue(sh, obj, []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_VALUE, nil),
	})
	if err != nil {
		return nil, err
	}
	if len(attrs) != 1 || len(attrs[0].Value) == 0 {
		return nil, fmt.Errorf("attribute value is empty")
	}
	out := make([]byte, len(attrs[0].Value))
	copy(out, attrs[0].Value)
	zero(attrs[0].Value)
	return out, nil
}

func (b *backend) readPublicKey(sh pkcs11.SessionHandle, obj pkcs11.ObjectHandle) (*ec.PublicKey, error) {
	attrs, err := b.module.GetAttributeValue(sh, obj, []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_EC_POINT, nil),
	})
	if err != nil {
		return nil, fmt.Errorf("read root public key: %w", err)
	}
	if len(attrs) != 1 {
		return nil, fmt.Errorf("read root public key: missing EC_POINT")
	}
	defer zero(attrs[0].Value)
	pub, err := parseECPoint(attrs[0].Value)
	if err != nil {
		return nil, fmt.Errorf("parse root public key: %w", err)
	}
	return pub, nil
}

func (b *backend) destroy(sh pkcs11.SessionHandle, obj pkcs11.ObjectHandle) {
	if obj == 0 {
		return
	}
	_ = b.module.DestroyObject(sh, obj)
}

func (b *backend) log(msg string) {
	if b.logf != nil {
		b.logf(msg)
	}
}

type hmacSession struct {
	b      *backend
	s      *sess
	scalar []byte
	once   sync.Once
}

func (h *hmacSession) ChildSign(digest []byte) ([]byte, error) {
	if len(digest) == 0 {
		return nil, fmt.Errorf("digest is empty")
	}
	pub, priv, err := h.keyPair()
	if err != nil {
		return nil, err
	}
	defer h.b.destroy(h.s.handle, pub)
	defer h.b.destroy(h.s.handle, priv)
	if err = h.b.module.SignInit(h.s.handle, []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_ECDSA, nil)}, priv); err != nil {
		return nil, fmt.Errorf("ecdsa init: %w", err)
	}
	raw, err := h.b.module.Sign(h.s.handle, digest)
	if err != nil {
		return nil, fmt.Errorf("ecdsa: %w", err)
	}
	return raw, nil
}

func (h *hmacSession) ChildSharedX(peerCompressed []byte) ([]byte, error) {
	if len(peerCompressed) != sharedPointLen {
		return nil, fmt.Errorf("child ecdh peer length %d, want %d", len(peerCompressed), sharedPointLen)
	}
	pub, priv, err := h.keyPair()
	if err != nil {
		return nil, err
	}
	defer h.b.destroy(h.s.handle, pub)
	defer h.b.destroy(h.s.handle, priv)
	secret, err := h.b.ecdh(h.s, priv, peerCompressed, true)
	if err != nil {
		return nil, fmt.Errorf("child ecdh: %w", err)
	}
	defer h.b.destroy(h.s.handle, secret)
	point, err := h.b.readValue(h.s.handle, secret)
	if err != nil {
		return nil, fmt.Errorf("child ecdh value: %w", err)
	}
	defer zero(point)
	if len(point) != sharedPointLen || (point[0] != 2 && point[0] != 3) {
		return nil, fmt.Errorf("child shared point length %d", len(point))
	}
	return new(big.Int).SetBytes(point[1:]).Bytes(), nil
}

func (h *hmacSession) keyPair() (pkcs11.ObjectHandle, pkcs11.ObjectHandle, error) {
	if h == nil || h.s == nil || len(h.scalar) != 32 {
		return 0, 0, fmt.Errorf("hmac session is closed")
	}
	pub, priv, err := deriveKeyPair(h.b.lib, h.s.handle, ckmECScalarChildDerive, h.scalar, h.s.root, childPublicTemplate(), childPrivateTemplate())
	if err != nil {
		return 0, 0, fmt.Errorf("scalar child derive: %w", err)
	}
	return pub, priv, nil
}

func (h *hmacSession) Close() error {
	if h == nil {
		return nil
	}
	h.once.Do(func() {
		zero(h.scalar)
		h.scalar = nil
		if h.b != nil && h.s != nil {
			h.b.pool.release(h.s)
		}
		h.s = nil
	})
	return nil
}

func secretTemplate(extractable bool) []*pkcs11.Attribute {
	return []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_SECRET_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_GENERIC_SECRET),
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, false),
		pkcs11.NewAttribute(pkcs11.CKA_SENSITIVE, !extractable),
		pkcs11.NewAttribute(pkcs11.CKA_EXTRACTABLE, extractable),
		pkcs11.NewAttribute(pkcs11.CKA_SIGN, true),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE_LEN, sharedPointLen),
	}
}

func childPublicTemplate() []*pkcs11.Attribute {
	return []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PUBLIC_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_EC),
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, false),
		pkcs11.NewAttribute(pkcs11.CKA_VERIFY, true),
	}
}

func childPrivateTemplate() []*pkcs11.Attribute {
	// DERIVE is set so BRC-43 can ECDH with the child without exporting it.
	// SIGN, SENSITIVE, and not EXTRACTABLE match the Grimsel template.
	return []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_EC),
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, false),
		pkcs11.NewAttribute(pkcs11.CKA_SENSITIVE, true),
		pkcs11.NewAttribute(pkcs11.CKA_EXTRACTABLE, false),
		pkcs11.NewAttribute(pkcs11.CKA_SIGN, true),
		pkcs11.NewAttribute(pkcs11.CKA_DERIVE, true),
	}
}

func findOne(module *pkcs11.Ctx, sh pkcs11.SessionHandle, class uint, label string) (pkcs11.ObjectHandle, error) {
	if err := module.FindObjectsInit(sh, []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, class),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
	}); err != nil {
		return 0, fmt.Errorf("find key: %w", err)
	}
	defer func() { _ = module.FindObjectsFinal(sh) }()
	objs, _, err := module.FindObjects(sh, 2)
	if err != nil {
		return 0, fmt.Errorf("find key: %w", err)
	}
	if len(objs) != 1 {
		return 0, fmt.Errorf("label %q matched %d objects of class %d", label, len(objs), class)
	}
	return objs[0], nil
}

func parseECPoint(der []byte) (*ec.PublicKey, error) {
	raw := der
	if len(der) > 2 && der[0] == 0x04 && int(der[1]) == len(der)-2 {
		raw = der[2:]
	}
	pub, err := ec.ParsePubKey(raw)
	if err != nil {
		return nil, err
	}
	return pub, nil
}

func readPIN(path string) ([]byte, error) {
	// The path comes from operator config. The file contents are the PIN and never appear in an error.
	buf, err := os.ReadFile(path) //nolint:gosec // PIN path is operator configuration, not a user-controlled archive
	if err != nil {
		return nil, fmt.Errorf("read pin file: %w", err)
	}
	buf = bytes.TrimSpace(buf)
	if len(buf) == 0 {
		return nil, fmt.Errorf("pin file is empty")
	}
	return buf, nil
}

func alreadyLoggedIn(err error) bool {
	var pkErr pkcs11.Error
	return errors.As(err, &pkErr) && pkErr == pkcs11.CKR_USER_ALREADY_LOGGED_IN
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

type pool struct {
	mu     sync.Mutex
	module *pkcs11.Ctx
	ch     chan *sess
	closed bool
	wg     sync.WaitGroup
}

type sess struct {
	pool   *pool
	module *pkcs11.Ctx
	handle pkcs11.SessionHandle
	root   pkcs11.ObjectHandle
	once   sync.Once
}

func newPool(module *pkcs11.Ctx, n int) *pool {
	return &pool{
		module: module,
		ch:     make(chan *sess, n),
	}
}

func (p *pool) add(s *sess) {
	p.wg.Add(1)
	p.ch <- s
}

func (p *pool) acquire(ctx context.Context) (*sess, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("pkcs11 session pool is closed")
	}
	ch := p.ch
	p.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case s, ok := <-ch:
		if !ok || s == nil {
			return nil, fmt.Errorf("pkcs11 session pool is closed")
		}
		return s, nil
	}
}

func (p *pool) release(s *sess) {
	if s == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		s.finish()
		return
	}
	p.ch <- s
}

func (p *pool) shutdown() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		p.wg.Wait()
		return
	}
	p.closed = true
	pending := len(p.ch)
	for i := 0; i < pending; i++ {
		s := <-p.ch
		s.finish()
	}
	close(p.ch)
	p.mu.Unlock()
	p.wg.Wait()
}

func (s *sess) finish() {
	s.once.Do(func() {
		if s.handle != 0 && s.module != nil {
			_ = s.module.Logout(s.handle)
			_ = s.module.CloseSession(s.handle)
		}
		s.pool.wg.Done()
	})
}

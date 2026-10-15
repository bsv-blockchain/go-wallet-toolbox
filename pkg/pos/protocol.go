// Package pos implements merchant-scoped nearby payment acceptance on top of
// a Go Wallet Toolbox signing wallet. It never exposes spending keys to cashiers.
package pos

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

var authorizationProtocol = sdk.Protocol{SecurityLevel: 2, Protocol: "bsv pos authorization"}

const (
	originator           = "pos.bsv"
	maxSafeInteger int64 = 9007199254740991
)

type Signer interface {
	GetPublicKey(context.Context, sdk.GetPublicKeyArgs, string) (*sdk.GetPublicKeyResult, error)
	CreateSignature(context.Context, sdk.CreateSignatureArgs, string) (*sdk.CreateSignatureResult, error)
}

// Signatures cover sorted JSON fields with Go/JS-compatible UTF-8 encoding.
// Numeric arrays deliberately avoid Go's base64 encoding of []byte.
type Signed struct {
	Payload     json.RawMessage `json:"payload"`
	IdentityKey string          `json:"identityKey"`
	Signature   []int           `json:"signature"`
}
type RPC struct {
	Version int             `json:"version"`
	Method  string          `json:"method"`
	Nonce   string          `json:"nonce"`
	Time    int64           `json:"time"`
	Body    json.RawMessage `json:"body"`
}
type Invite struct {
	Kind     string `json:"kind"`
	Merchant string `json:"merchant"`
	Name     string `json:"name"`
	Service  string `json:"service"`
	Chain    string `json:"chain"`
	ID       string `json:"id"`
	Expires  int64  `json:"expires"`
}
type Request struct {
	Subtotal int64  `json:"subtotal,omitempty"`
	Tip      int64  `json:"tip,omitempty"`
	Kind     string `json:"kind"`
	Merchant string `json:"merchant"`
	Name     string `json:"name"`
	Cashier  string `json:"cashier"`
	Invoice  string `json:"invoice"`
	Chain    string `json:"chain"`
	Expires  int64  `json:"expires"`
	Session  string `json:"session"`
}
type Receipt struct {
	Invoice string `json:"invoice"`
	TxID    string `json:"txid"`
	Amount  int64  `json:"amount"`
	Status  string `json:"status"`
}
type Frame struct {
	Version           int             `json:"version"`
	Kind              string          `json:"kind"`
	SenderIdentityKey string          `json:"senderIdentityKey"`
	OutputIndex       uint32          `json:"outputIndex"`
	DerivationPrefix  string          `json:"derivationPrefix"`
	DerivationSuffix  string          `json:"derivationSuffix"`
	Transaction       []int           `json:"transaction"`
	Note              string          `json:"note,omitempty"`
	Token             json.RawMessage `json:"token,omitempty"`
}
type Session struct {
	Version  int             `json:"v"`
	Caps     uint32          `json:"c"`
	ID       string          `json:"s"`
	PSK      string          `json:"k"`
	Identity string          `json:"i"`
	Amount   int64           `json:"a"`
	Prefix   string          `json:"p"`
	Suffix   string          `json:"x"`
	Asset    json.RawMessage `json:"t,omitempty"`
}

func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

func identity(value string) (*ec.PublicKey, error) {
	if len(value) != 66 || (!strings.HasPrefix(value, "02") && !strings.HasPrefix(value, "03")) {
		return nil, errors.New("invalid identity")
	}
	return ec.PublicKeyFromString(value)
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func Sign(ctx context.Context, wallet Signer, payload any) (*Signed, error) {
	raw, err := canonical(payload)
	if err != nil {
		return nil, err
	}
	public, err := wallet.GetPublicKey(ctx, sdk.GetPublicKeyArgs{IdentityKey: true}, originator)
	if err != nil {
		return nil, err
	}
	result, err := wallet.CreateSignature(ctx, sdk.CreateSignatureArgs{EncryptionArgs: sdk.EncryptionArgs{ProtocolID: authorizationProtocol, KeyID: "v1", Counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeAnyone}}, Data: raw}, originator)
	if err != nil {
		return nil, err
	}
	if result == nil || result.Signature == nil || public == nil || public.PublicKey == nil {
		return nil, errors.New("wallet signature unavailable")
	}
	signature := result.Signature.Serialize()
	numbers := make([]int, len(signature))
	for i, b := range signature {
		numbers[i] = int(b)
	}
	return &Signed{Payload: raw, IdentityKey: strings.ToLower(public.PublicKey.ToDERHex()), Signature: numbers}, nil
}

func Verify(ctx context.Context, value Signed) error {
	public, err := identity(value.IdentityKey)
	if err != nil {
		return err
	}
	signatureBytes, err := byteArray(value.Signature, 80)
	if err != nil {
		return err
	}
	signature, err := ec.ParseDERSignature(signatureBytes)
	if err != nil {
		return err
	}
	canonicalBytes, err := canonical(value.Payload)
	if err != nil {
		return err
	}
	wallet, err := sdk.NewProtoWallet(sdk.ProtoWalletArgs{Type: sdk.ProtoWalletArgsTypeAnyone})
	if err != nil {
		return err
	}
	result, err := wallet.VerifySignature(ctx, sdk.VerifySignatureArgs{EncryptionArgs: sdk.EncryptionArgs{ProtocolID: authorizationProtocol, KeyID: "v1", Counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: public}}, Data: canonicalBytes, Signature: signature}, originator)
	if err != nil {
		return err
	}
	if result == nil || !result.Valid {
		return errors.New("invalid signature")
	}
	return nil
}

func byteArray(numbers []int, limit int) ([]byte, error) {
	if len(numbers) == 0 || len(numbers) > limit {
		return nil, errors.New("invalid byte array length")
	}
	b := make([]byte, len(numbers))
	for i, n := range numbers {
		if n < 0 || n > 255 {
			return nil, errors.New("invalid byte")
		}
		b[i] = byte(n)
	}
	return b, nil
}

func decodeSession(text string) (Session, error) {
	var s Session
	if !strings.HasPrefix(text, "bsvpay1:") || len(text) > 4096 {
		return s, errors.New("invalid session")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(text, "bsvpay1:"))
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(raw, &s); err != nil {
		return s, err
	}
	if s.Version != 1 || s.Amount <= 0 || s.Amount > maxSafeInteger || len(s.Asset) > 0 {
		return s, errors.New("invalid payment request")
	}
	if _, err = identity(s.Identity); err != nil {
		return s, err
	}
	id, err := base64.RawURLEncoding.DecodeString(s.ID)
	if err != nil || len(id) != 16 {
		return s, errors.New("invalid session id")
	}
	psk, err := base64.RawURLEncoding.DecodeString(s.PSK)
	if err != nil || len(psk) != 32 {
		return s, errors.New("invalid session key")
	}
	// Go SDK remittance stores decoded nonce bytes then maps them to base64.
	// Require canonical base64 here so both SDKs derive from identical strings.
	for _, v := range []string{s.Prefix, s.Suffix} {
		b, e := base64.StdEncoding.DecodeString(v)
		if e != nil || len(b) == 0 || len(b) > 128 || base64.StdEncoding.EncodeToString(b) != v {
			return s, errors.New("invalid derivation nonce")
		}
	}
	return s, nil
}

func validateURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("merchant service must use HTTPS")
	}
	return nil
}

func canonical(payload any) ([]byte, error) {
	raw, err := marshal(payload)
	if err != nil {
		return nil, err
	}
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return marshal(decoded)
}

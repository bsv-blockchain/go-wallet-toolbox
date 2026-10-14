package pos

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func testSigner(t *testing.T, hex string) *sdk.ProtoWallet {
	t.Helper()
	p, e := ec.PrivateKeyFromHex(hex)
	if e != nil {
		t.Fatal(e)
	}
	w, e := sdk.NewProtoWallet(sdk.ProtoWalletArgs{Type: sdk.ProtoWalletArgsTypePrivateKey, PrivateKey: p})
	if e != nil {
		t.Fatal(e)
	}
	return w
}

func call(t *testing.T, s *Service, w Signer, method string, body any) (any, error) {
	t.Helper()
	raw, _ := marshal(body)
	signed, err := Sign(context.Background(), w, RPC{1, method, randomID(), time.Now().UnixMilli(), raw})
	if err != nil {
		t.Fatal(err)
	}
	return s.Call(context.Background(), *signed)
}

func signerIdentity(t *testing.T, w Signer) string {
	t.Helper()
	p, e := w.GetPublicKey(context.Background(), sdk.GetPublicKeyArgs{IdentityKey: true}, originator)
	if e != nil {
		t.Fatal(e)
	}
	return p.PublicKey.ToDERHex()
}

func sessionText(merchant string) string {
	raw, _ := marshal(map[string]any{"v": 1, "c": 0, "s": base64.RawURLEncoding.EncodeToString(make([]byte, 16)), "k": base64.RawURLEncoding.EncodeToString(make([]byte, 32)), "i": merchant, "a": 123, "p": "cHJlZml4", "x": "c3VmZml4"})
	return "bsvpay1:" + base64.RawURLEncoding.EncodeToString(raw)
}

func TestEnrollmentAndIdempotentSettlement(t *testing.T) {
	ctx := context.Background()
	owner, cashier, other := testSigner(t, "01"), testSigner(t, "02"), testSigner(t, "03")
	merchant := signerIdentity(t, owner)
	store := FileStore{filepath.Join(t.TempDir(), "state.json")}
	attempts := 0
	fail := true
	config := Config{Wallet: owner, Merchant: merchant, Name: "Corner Cafe", URL: "https://merchant.example", Chain: "test", Store: store, Accept: func(ctx context.Context, f Frame, amount int64) (string, error) {
		attempts++
		if amount != 123 {
			t.Fatal("amount not bound")
		}
		if fail {
			return "", errors.New("lost wallet response")
		}
		return strings.Repeat("a", 64), nil
	}}
	s, e := New(config)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = call(t, s, cashier, "invite", map[string]any{}); e == nil {
		t.Fatal("cashier minted invitation")
	}
	inv, e := call(t, s, owner, "invite", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	var invitation Invite
	if e = json.Unmarshal(inv.(*Signed).Payload, &invitation); e != nil {
		t.Fatal(e)
	}
	if _, e = call(t, s, cashier, "enroll", map[string]any{"invite": invitation.ID, "name": "Sam"}); e != nil {
		t.Fatal(e)
	}
	if _, e = call(t, s, cashier, "invoice", map[string]any{"session": sessionText(merchant)}); e == nil {
		t.Fatal("unapproved cashier invoiced")
	}
	if _, e = call(t, s, owner, "approve", map[string]any{"cashier": signerIdentity(t, cashier)}); e != nil {
		t.Fatal(e)
	}
	req, e := call(t, s, cashier, "invoice", map[string]any{"session": sessionText(merchant)})
	if e != nil {
		t.Fatal(e)
	}
	if e = Verify(ctx, *req.(*Signed)); e != nil {
		t.Fatal(e)
	}
	var terms Request
	if e = json.Unmarshal(req.(*Signed).Payload, &terms); e != nil {
		t.Fatal(e)
	}
	frame := Frame{Version: 5, Kind: "bsv", SenderIdentityKey: signerIdentity(t, other), OutputIndex: 0, DerivationPrefix: "cHJlZml4", DerivationSuffix: "c3VmZml4", Transaction: []int{1, 2, 3}}
	if _, e = call(t, s, other, "settle", map[string]any{"invoice": terms.Invoice, "frame": frame}); e == nil {
		t.Fatal("other cashier settled")
	}
	bad := frame
	bad.DerivationSuffix = "Zm9yZ2Vk"
	if _, e = call(t, s, cashier, "settle", map[string]any{"invoice": terms.Invoice, "frame": bad}); e == nil {
		t.Fatal("unbound payment accepted")
	}
	if attempts != 0 {
		t.Fatal("wallet touched for invalid frame")
	}
	if _, e = call(t, s, cashier, "settle", map[string]any{"invoice": terms.Invoice, "frame": frame}); e == nil {
		t.Fatal("wallet failure acknowledged")
	}
	saved, e := store.Load(ctx)
	if e != nil || saved.Sales[terms.Invoice].Frame == nil || saved.Sales[terms.Invoice].Receipt != nil {
		t.Fatal("pending payment not persisted")
	}
	// Restart after the uncertain acceptance; retry only those original bytes.
	s, e = New(config)
	if e != nil {
		t.Fatal(e)
	}
	fail = false
	receipt, e := call(t, s, cashier, "settle", map[string]any{"invoice": terms.Invoice, "frame": frame})
	if e != nil {
		t.Fatal(e)
	}
	if e = Verify(ctx, *receipt.(*Signed)); e != nil {
		t.Fatal(e)
	}
	count := attempts
	if _, e = call(t, s, cashier, "settle", map[string]any{"invoice": terms.Invoice, "frame": frame}); e != nil {
		t.Fatal(e)
	}
	if attempts != count {
		t.Fatal("wallet invoked after receipt")
	}
	bad = frame
	bad.Transaction = []int{4, 5, 6}
	if _, e = call(t, s, cashier, "settle", map[string]any{"invoice": terms.Invoice, "frame": bad}); e == nil {
		t.Fatal("different transaction reused invoice")
	}
	if _, e = call(t, s, owner, "revoke", map[string]any{"cashier": signerIdentity(t, cashier)}); e != nil {
		t.Fatal(e)
	}
	if _, e = call(t, s, cashier, "invoice", map[string]any{"session": sessionText(merchant)}); e == nil {
		t.Fatal("revoked cashier invoiced")
	}
}

func TestSignatureReplayAndTampering(t *testing.T) {
	owner := testSigner(t, "01")
	s, e := New(Config{Wallet: owner, Merchant: signerIdentity(t, owner), Name: "Cafe", URL: "https://merchant.example", Chain: "test", Store: FileStore{filepath.Join(t.TempDir(), "state.json")}, Accept: func(context.Context, Frame, int64) (string, error) { t.Fatal("unexpected settlement"); return "", nil }})
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := marshal(map[string]any{})
	signed, e := Sign(context.Background(), owner, RPC{1, "info", randomID(), time.Now().UnixMilli(), raw})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Call(context.Background(), *signed); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Call(context.Background(), *signed); e == nil {
		t.Fatal("replay allowed")
	}
	tampered := *signed
	tampered.Payload = []byte(strings.Replace(string(signed.Payload), "info", "invite", 1))
	if e = Verify(context.Background(), tampered); e == nil {
		t.Fatal("tampered signature accepted")
	}
}

func TestInvalidSession(t *testing.T) {
	merchant := signerIdentity(t, testSigner(t, "01"))
	session, e := decodeSession(sessionText(merchant))
	if e != nil || session.Amount != 123 {
		t.Fatal(e)
	}
	if _, e = decodeSession("bsvpay1:e30"); e == nil {
		t.Fatal("empty session accepted")
	}
	if _, e = byteArray([]int{256}, 10); e == nil {
		t.Fatal("overflow byte accepted")
	}
}

func TestTypeScriptSignatureVector(t *testing.T) {
	raw, err := os.ReadFile("testdata/typescript-signature.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector Signed
	if err = json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	if err = Verify(context.Background(), vector); err != nil {
		t.Fatal("TypeScript signature failed in Go:", err)
	}
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(vector.Payload))
	decoder.UseNumber()
	if err = decoder.Decode(&payload); err != nil {
		t.Fatal(err)
	}
	result, err := Sign(context.Background(), testSigner(t, "01"), payload)
	if err != nil {
		t.Fatal(err)
	}
	// Both SDKs use deterministic signatures and the same canonical bytes.
	if !reflect.DeepEqual(result.Signature, vector.Signature) {
		t.Fatal("Go and TypeScript signature bytes differ")
	}
}

func TestConcurrentDeliveriesAcceptOnce(t *testing.T) {
	owner, cashier := testSigner(t, "01"), testSigner(t, "02")
	merchant := signerIdentity(t, owner)
	calls := 0
	s, err := New(Config{Wallet: owner, Merchant: merchant, Name: "Cafe", URL: "https://merchant.example", Chain: "test", Store: FileStore{filepath.Join(t.TempDir(), "state.json")}, Accept: func(context.Context, Frame, int64) (string, error) { calls++; return strings.Repeat("a", 64), nil }})
	if err != nil {
		t.Fatal(err)
	}
	inv, err := call(t, s, owner, "invite", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	var invitation Invite
	if err = json.Unmarshal(inv.(*Signed).Payload, &invitation); err != nil {
		t.Fatal(err)
	}
	if _, err = call(t, s, cashier, "enroll", map[string]any{"invite": invitation.ID, "name": "Sam"}); err != nil {
		t.Fatal(err)
	}
	if _, err = call(t, s, owner, "approve", map[string]any{"cashier": signerIdentity(t, cashier)}); err != nil {
		t.Fatal(err)
	}
	req, err := call(t, s, cashier, "invoice", map[string]any{"session": sessionText(merchant)})
	if err != nil {
		t.Fatal(err)
	}
	var terms Request
	if err = json.Unmarshal(req.(*Signed).Payload, &terms); err != nil {
		t.Fatal(err)
	}
	frame := Frame{Version: 5, Kind: "bsv", SenderIdentityKey: signerIdentity(t, owner), DerivationPrefix: "cHJlZml4", DerivationSuffix: "c3VmZml4", Transaction: []int{1}}
	body, _ := marshal(map[string]any{"invoice": terms.Invoice, "frame": frame})
	requests := make([]*Signed, 8)
	for i := range requests {
		requests[i], err = Sign(context.Background(), cashier, RPC{1, "settle", randomID(), time.Now().UnixMilli(), body})
		if err != nil {
			t.Fatal(err)
		}
	}
	results := make(chan error, len(requests))
	for _, request := range requests {
		go func(request *Signed) { _, err := s.Call(context.Background(), *request); results <- err }(request)
	}
	for range requests {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("wallet accepted %d times, want one", calls)
	}
}

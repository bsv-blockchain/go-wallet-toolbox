package pos

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

const sessionBodyKey = "session"

func TestInvoiceBindsTipAndSubtotal(t *testing.T) {
	ctx := t.Context()
	owner, cashier := testSigner(t, "01"), testSigner(t, "02")
	merchant, cashierID := signerIdentity(t, owner), signerIdentity(t, cashier)
	store := FileStore{Path: filepath.Join(t.TempDir(), "state.json")}
	state := EmptyState()
	state.Cashiers[cashierID] = &Cashier{Identity: cashierID, Name: "Cashier", State: "active"}
	if err := store.Save(ctx, state); err != nil {
		t.Fatal(err)
	}
	service, err := New(Config{
		Wallet: owner, Merchant: merchant, Name: "Cafe", URL: "https://merchant.example", Chain: "test", Store: store,
		Accept: func(context.Context, Frame, int64) (string, error) { return "", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	session := sessionText(merchant)
	for _, body := range []map[string]any{
		{sessionBodyKey: session, "subtotal": 100, "tip": 22},
		{sessionBodyKey: session, "subtotal": 124, "tip": -1},
		{sessionBodyKey: session, "subtotal": 0, "tip": 123},
		{sessionBodyKey: session, "subtotal": 100},
		{sessionBodyKey: session, "tip": 23},
		{sessionBodyKey: session, "subtotal": 100, "tip": 9223372036854775807},
	} {
		if _, callErr := call(t, service, cashier, "invoice", body); callErr == nil {
			t.Fatal("invalid breakdown accepted")
		}
	}
	result, err := call(t, service, cashier, "invoice", map[string]any{sessionBodyKey: session, "subtotal": 100, "tip": 23})
	if err != nil {
		t.Fatal(err)
	}
	envelope := result.(*Signed)
	if err = Verify(ctx, *envelope); err != nil {
		t.Fatal(err)
	}
	var request Request
	if err = json.Unmarshal(envelope.Payload, &request); err != nil {
		t.Fatal(err)
	}
	if request.Subtotal != 100 || request.Tip != 23 {
		t.Fatal("tip breakdown not retained")
	}
	tampered := *envelope
	request.Tip = 24
	tampered.Payload, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err = Verify(ctx, tampered); err == nil {
		t.Fatal("modified tip retained a valid signature")
	}
	legacy, err := call(t, service, cashier, "invoice", map[string]any{sessionBodyKey: session})
	if err != nil {
		t.Fatal("legacy request rejected", err)
	}
	if err = Verify(ctx, *legacy.(*Signed)); err != nil {
		t.Fatal(err)
	}
}

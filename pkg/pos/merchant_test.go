package pos

import (
	"context"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"testing"
)

type acceptingWallet struct {
	*sdk.ProtoWallet
	accepted     bool
	status       sdk.ActionStatus
	txid         string
	internalized int
}

func (w *acceptingWallet) InternalizeAction(ctx context.Context, args sdk.InternalizeActionArgs, origin string) (*sdk.InternalizeActionResult, error) {
	w.internalized++
	return &sdk.InternalizeActionResult{Accepted: w.accepted}, nil
}
func (w *acceptingWallet) ListActions(ctx context.Context, args sdk.ListActionsArgs, origin string) (*sdk.ListActionsResult, error) {
	hash, e := chainhash.NewHashFromHex(w.txid)
	if e != nil {
		return nil, e
	}
	return &sdk.ListActionsResult{Actions: []sdk.Action{{Txid: *hash, Status: w.status}}}, nil
}
func TestMerchantAcceptanceRequiresDestinationAmountAndBroadcast(t *testing.T) {
	ctx := context.Background()
	owner := testSigner(t, "01")
	sender := testSigner(t, "02")
	senderKey := signerIdentity(t, sender)
	pub, err := identity(senderKey)
	if err != nil {
		t.Fatal(err)
	}
	own := true
	derived, err := owner.GetPublicKey(ctx, sdk.GetPublicKeyArgs{EncryptionArgs: sdk.EncryptionArgs{ProtocolID: sdk.Protocol{SecurityLevel: 2, Protocol: "3241645161d8"}, KeyID: "cHJlZml4 c3VmZml4", Counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: pub}}, ForSelf: &own}, originator)
	if err != nil {
		t.Fatal(err)
	}
	address, err := script.NewAddressFromPublicKey(derived.PublicKey, true)
	if err != nil {
		t.Fatal(err)
	}
	locking, err := p2pkh.Lock(address)
	if err != nil {
		t.Fatal(err)
	}
	tx := transaction.NewTransaction()
	tx.Outputs = []*transaction.TransactionOutput{{Satoshis: 123, LockingScript: locking}}
	raw, err := tx.AtomicBEEF(false)
	if err != nil {
		t.Fatal(err)
	}
	numbers := make([]int, len(raw))
	for i, b := range raw {
		numbers[i] = int(b)
	}
	f := Frame{Version: 5, Kind: "bsv", SenderIdentityKey: senderKey, DerivationPrefix: "cHJlZml4", DerivationSuffix: "c3VmZml4", Transaction: numbers}
	w := &acceptingWallet{ProtoWallet: owner, accepted: true, status: sdk.ActionStatusSending, txid: tx.TxID().String()}
	accept := MerchantAcceptor(w)
	if _, err = accept(ctx, f, 124); err == nil {
		t.Fatal("wrong amount accepted")
	}
	if w.internalized != 0 {
		t.Fatal("wrong amount touched wallet")
	}
	bad := f
	bad.DerivationSuffix = "Zm9yZ2Vk"
	if _, err = accept(ctx, bad, 123); err == nil {
		t.Fatal("wrong destination accepted")
	}
	if w.internalized != 0 {
		t.Fatal("wrong destination touched wallet")
	}
	if _, err = accept(ctx, f, 123); err == nil {
		t.Fatal("pending broadcast accepted")
	}
	w.status = sdk.ActionStatusUnproven
	w.accepted = false
	if _, err = accept(ctx, f, 123); err == nil {
		t.Fatal("wallet refusal ignored")
	}
	w.accepted = true
	if got, err := accept(ctx, f, 123); err != nil || got != w.txid {
		t.Fatal("accepted payment failed", got, err)
	}
}

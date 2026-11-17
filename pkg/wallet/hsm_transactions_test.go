package wallet_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script/interpreter"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/mocks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/tsgenerated"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Embedding the interface makes any unexpected storage call fail the test.
type hsmTransactionStorage struct {
	wdk.WalletStorage
	create       *wdk.StorageCreateActionResult
	processed    []wdk.ProcessActionArgs
	internalized int
}

func (s *hsmTransactionStorage) CreateAction(context.Context, wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error) {
	return s.create, nil
}
func (s *hsmTransactionStorage) ProcessAction(_ context.Context, args wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error) {
	s.processed = append(s.processed, args)
	return &wdk.ProcessActionResult{}, nil
}
func (s *hsmTransactionStorage) InternalizeAction(context.Context, wdk.InternalizeActionArgs) (*wdk.InternalizeActionResult, error) {
	s.internalized++
	return &wdk.InternalizeActionResult{Accepted: true}, nil
}

// Use the same optional fast path as the real HSM deriver and reject expired
// request contexts so deferred signing cannot accidentally pass this test.
type transactionDeriver struct {
	*sdkDeriver
	signs int
}

func (s *transactionDeriver) SignWithPublicKey(ctx context.Context, p sdk.Protocol, id string, cp sdk.Counterparty, digest []byte) (*ec.Signature, *ec.PublicKey, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	s.signs++
	sig, err := s.sdkDeriver.Sign(ctx, p, id, cp, digest)
	if err != nil {
		return nil, nil, err
	}
	pub, err := s.kd.DerivePublicKey(p, id, cp, true)
	return sig, pub, err
}

func TestNewWithHSMTransactionActions(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		name := "CreateAction"
		if deferred {
			name = "SignActionAfterCanceledCreate"
		}
		t.Run(name, func(t *testing.T) {
			runHSMTransactionAction(t, deferred)
		})
	}
}

func runHSMTransactionAction(t *testing.T, deferred bool) {
	t.Helper()
	created, parents := hsmActionFixture(t)
	store := &hsmTransactionStorage{create: created}
	signer := &transactionDeriver{sdkDeriver: newSDKDeriver(t, testusers.Alice.PrivateKey(t))}
	w, err := wallet.NewWithHSM(defs.NetworkMainnet, signer, mocks.NewMockWalletStorageProvider(nil), wallet.WithStorageManager(store))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	immediate := !deferred
	args := sdk.CreateActionArgs{Description: "HSM transaction test", Outputs: []sdk.CreateActionOutput{{LockingScript: []byte{0x51}, Satoshis: 1000, OutputDescription: "test output"}}, Options: &sdk.CreateActionOptions{SignAndProcess: &immediate}}
	result, err := w.CreateAction(ctx, args, fixtures.DefaultOriginator)
	require.NoError(t, err)
	if deferred {
		require.NotNil(t, result.SignableTransaction)
		require.Empty(t, store.processed)
		require.Zero(t, signer.signs)
		cancel()
		_, err = w.SignAction(t.Context(), sdk.SignActionArgs{Reference: result.SignableTransaction.Reference}, fixtures.DefaultOriginator)
		require.NoError(t, err)
	}
	require.Len(t, store.processed, 1)
	tx, err := transaction.NewTransactionFromBytes(store.processed[0].RawTx)
	require.NoError(t, err)
	require.Equal(t, len(created.Inputs), signer.signs)
	requireInputsSpend(t, tx, parents)
	requireChangeLockedToAlice(t, tx, created)
}

func hsmActionFixture(t *testing.T) (*wdk.StorageCreateActionResult, map[string]*transaction.Transaction) {
	t.Helper()
	var created wdk.StorageCreateActionResult
	require.NoError(t, json.Unmarshal([]byte(tsgenerated.CreateActionResultJSON()), &created))
	previous, err := chainhash.NewHashFromHex("0000000000000000000000000000000000000000000000000000000000000001")
	require.NoError(t, err)
	kd := testusers.Alice.KeyDeriver(t)
	parents := make(map[string]*transaction.Transaction)
	for _, in := range created.Inputs {
		attachBRC29Parent(t, kd, previous, in, parents)
	}
	return &created, parents
}

func attachBRC29Parent(t *testing.T, kd *sdk.KeyDeriver, previous *chainhash.Hash, in *wdk.StorageCreateTransactionSdkInput, parents map[string]*transaction.Transaction) {
	t.Helper()
	sender, err := ec.PublicKeyFromString(*in.SenderIdentityKey)
	require.NoError(t, err)
	lock, err := brc29.LockForSelf(sender, brc29.KeyID{DerivationPrefix: *in.DerivationPrefix, DerivationSuffix: *in.DerivationSuffix}, kd)
	require.NoError(t, err)
	satoshis, err := to.UInt64(in.SourceSatoshis)
	require.NoError(t, err)
	parent := transaction.NewTransaction()
	parent.AddInput(&transaction.TransactionInput{SourceTXID: previous, SequenceNumber: transaction.DefaultSequenceNumber})
	for vout := uint32(0); vout <= in.SourceVout; vout++ {
		parent.AddOutput(&transaction.TransactionOutput{Satoshis: satoshis, LockingScript: lock})
	}
	in.SourceLockingScript = lock.String()
	in.SourceTxID = parent.TxID().String()
	in.SourceTransaction = parent.Bytes()
	parents[in.SourceTxID] = parent
}

func requireInputsSpend(t *testing.T, tx *transaction.Transaction, parents map[string]*transaction.Transaction) {
	t.Helper()
	for i, in := range tx.Inputs {
		in.SourceTransaction = parents[in.SourceTXID.String()]
		require.NoError(t, interpreter.NewEngine().Execute(interpreter.WithTx(tx, i, in.SourceTxOutput()), interpreter.WithAfterGenesis(), interpreter.WithForkID()))
	}
}

func requireChangeLockedToAlice(t *testing.T, tx *transaction.Transaction, created *wdk.StorageCreateActionResult) {
	t.Helper()
	kd := testusers.Alice.KeyDeriver(t)
	changes := 0
	for _, out := range created.Outputs {
		if out.DerivationSuffix == nil {
			continue
		}
		want, err := brc29.LockForSelf(kd.IdentityKey(), brc29.KeyID{DerivationPrefix: created.DerivationPrefix, DerivationSuffix: *out.DerivationSuffix}, kd)
		require.NoError(t, err)
		require.Equal(t, want.String(), tx.Outputs[out.Vout].LockingScript.String())
		changes++
	}
	require.Positive(t, changes)
}

func TestNewWithHSMInternalizeAction(t *testing.T) {
	store := &hsmTransactionStorage{}
	w, err := wallet.NewWithHSM(defs.NetworkMainnet, newSDKDeriver(t, testusers.Alice.PrivateKey(t)), mocks.NewMockWalletStorageProvider(nil), wallet.WithStorageManager(store))
	require.NoError(t, err)
	args := fixtures.DefaultWalletInternalizeActionArgsMatchingBRC29(t, sdk.InternalizeProtocolWalletPayment, testusers.Alice.KeyDeriver(t))
	result, err := w.InternalizeAction(t.Context(), args, fixtures.DefaultOriginator)
	require.NoError(t, err)
	require.True(t, result.Accepted)
	require.Equal(t, 1, store.internalized)
	args.Outputs[0].PaymentRemittance.DerivationSuffix = []byte("wrong")
	_, err = w.InternalizeAction(t.Context(), args, fixtures.DefaultOriginator)
	require.Error(t, err)
	require.Equal(t, 1, store.internalized, "invalid remittance must not reach storage")
}

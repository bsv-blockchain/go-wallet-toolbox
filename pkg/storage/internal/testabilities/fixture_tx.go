package testabilities

import (
	"fmt"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

type TxGeneratorFixture interface {
	WithSatoshisToInternalize(satoshis uint64) TxGeneratorFixture
	WithSatoshisToSend(satoshis uint64) TxGeneratorFixture
	WithSender(sender testusers.User) TxGeneratorFixture
	WithRecipient(recipient testusers.User) TxGeneratorFixture
	WithDelayedBroadcast() TxGeneratorFixture
	WithReference(reference string) TxGeneratorFixture
	WithLabels(labels ...string) TxGeneratorFixture
	WillFailOnBroadcast() TxGeneratorFixture

	PreInternalized() (internalizeArgs *wdk.InternalizeActionArgs, toInternalize *transaction.Transaction)
	Internalized() (internalizeResult *wdk.InternalizeActionResult, internalizedTx *transaction.Transaction)
	Created() (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction)
	Processed() (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction)
	Unprocessed() (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction)
}

type txGeneratorFixture struct {
	testing.TB

	parent                *storageFixture
	satoshisToInternalize uint64
	satoshisToSend        uint64
	activeStorage         *storage.Provider
	sender                testusers.User
	recipient             testusers.User
	delayedBroadcast      bool
	failedBroadcast       bool
	reference             string
	labels                []string
}

func (t *txGeneratorFixture) WithSatoshisToInternalize(satoshis uint64) TxGeneratorFixture {
	t.satoshisToInternalize = satoshis
	return t
}

func (t *txGeneratorFixture) WithSatoshisToSend(satoshis uint64) TxGeneratorFixture {
	t.satoshisToSend = satoshis
	return t
}

func (t *txGeneratorFixture) WithSender(sender testusers.User) TxGeneratorFixture {
	t.sender = sender
	return t
}

func (t *txGeneratorFixture) WithRecipient(recipient testusers.User) TxGeneratorFixture {
	t.recipient = recipient
	return t
}

func (t *txGeneratorFixture) WithDelayedBroadcast() TxGeneratorFixture {
	t.delayedBroadcast = true
	return t
}

func (t *txGeneratorFixture) WithReference(reference string) TxGeneratorFixture {
	t.reference = reference
	return t
}

func (t *txGeneratorFixture) WithLabels(labels ...string) TxGeneratorFixture {
	t.labels = append(t.labels, labels...)
	return t
}

func (t *txGeneratorFixture) WillFailOnBroadcast() TxGeneratorFixture {
	t.failedBroadcast = true
	return t
}

func (t *txGeneratorFixture) PreInternalized() (internalizeArgs *wdk.InternalizeActionArgs, toInternalize *transaction.Transaction) {
	t.Helper()
	keyID := brc29.KeyID{
		DerivationPrefix: fixtures.DerivationPrefix,
		DerivationSuffix: fixtures.DerivationSuffix,
	}

	anyonePriv, anyonePub := sdk.AnyoneKey()

	address, err := brc29.AddressForCounterparty(anyonePriv, keyID, t.sender.PublicKey(t), brc29.WithTestNet())
	require.NoError(t.TB, err)

	lockingScript, err := p2pkh.Lock(address)
	require.NoError(t.TB, err)

	spec := testvectors.GivenTX().
		WithInput(t.satoshisToInternalize+1).
		WithOutputScript(t.satoshisToInternalize, lockingScript)

	internalizeArgs = &wdk.InternalizeActionArgs{
		Tx: spec.AtomicBEEF().Bytes(),
		Outputs: []*wdk.InternalizeOutput{
			{
				OutputIndex: 0,
				Protocol:    wdk.WalletPaymentProtocol,
				PaymentRemittance: &wdk.WalletPayment{
					DerivationPrefix:  fixtures.DerivationPrefix,
					DerivationSuffix:  fixtures.DerivationSuffix,
					SenderIdentityKey: primitives.PubKeyHex(anyonePub.ToDERHex()),
				},
			},
		},
		Description: "description",
	}

	beef, err := transaction.NewBeefFromTransaction(spec.TX())
	require.NoError(t, err)

	require.Len(t, beef.BUMPs, 1)
	bump := beef.BUMPs[0]
	merkleRoot, err := bump.ComputeRoot(spec.ID())
	require.NoError(t, err)

	t.parent.Provider().BHS().OnMerkleRootVerifyResponse(
		bump.BlockHeight,
		merkleRoot.String(),
		BHSMerkleRootConfirmed,
	)

	return internalizeArgs, spec.TX()
}

func (t *txGeneratorFixture) Internalized() (internalizeResult *wdk.InternalizeActionResult, internalizedTx *transaction.Transaction) {
	t.Helper()
	internalizeArgs, internalizedTx := t.PreInternalized()

	result, err := t.activeStorage.InternalizeAction(t.Context(), t.sender.AuthID(), *internalizeArgs)
	require.NoError(t, err)

	return result, internalizedTx
}

func (t *txGeneratorFixture) Created() (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction) {
	t.Helper()
	_, parentTx := t.Internalized()

	keyID := brc29.KeyID{
		DerivationPrefix: fixtures.DerivationPrefix,
		DerivationSuffix: fixtures.DerivationSuffix,
	}
	address, err := brc29.AddressForCounterparty(t.sender.PrivateKey(t), keyID, t.recipient.PublicKey(t), brc29.WithTestNet())
	require.NoError(t.TB, err)

	lockingScript, err := p2pkh.Lock(address)
	require.NoError(t.TB, err)

	args := wdk.ValidCreateActionArgs{
		Description: "outputBRC29",
		Labels:      []primitives.StringUnder300{fixtures.CreateActionTestLabel},
	}
	if t.reference != "" {
		args.Labels = append(args.Labels, primitives.StringUnder300(t.reference))
	}
	args.Inputs = []wdk.ValidCreateActionInput{
		{
			Outpoint: wdk.OutPoint{
				TxID: parentTx.TxID().String(),
				Vout: 0,
			},
			InputDescription:      "provided by previously internalized transaction",
			UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
		},
	}
	args.Outputs = []wdk.ValidCreateActionOutput{
		{
			LockingScript:      primitives.HexString(lockingScript.String()),
			Satoshis:           primitives.SatoshiValue(t.satoshisToSend),
			OutputDescription:  "output sent to Bob",
			CustomInstructions: to.Ptr(fmt.Sprintf(`{"derivationPrefix":"%s","derivationSuffix":"%s","type":"BRC29"}`, fixtures.DerivationPrefix, fixtures.DerivationSuffix)),
			Tags:               []primitives.StringUnder300{fixtures.CreateActionTestTag},
		},
	}
	args.LockTime = 0
	args.Version = 1
	if len(t.labels) > 0 {
		args.Labels = append(args.Labels, primitives.ToStringUnder300Slice(t.labels)...)
	}
	args.Options = wdk.ValidCreateActionOptions{
		AcceptDelayedBroadcast: to.Ptr(primitives.BooleanDefaultTrue(t.delayedBroadcast)),
		SendWith:               []primitives.TXIDHexString{},
		SignAndProcess:         to.Ptr(primitives.BooleanDefaultTrue(true)),
		KnownTxids:             []primitives.TXIDHexString{},
		NoSendChange:           []wdk.OutPoint{},
		RandomizeOutputs:       false,
		TrustSelf:              to.Ptr(sdk.TrustSelfKnown),
	}
	args.IsSendWith = false
	args.IsDelayed = t.delayedBroadcast
	args.IsNoSend = false
	args.IsNewTx = true
	args.IsRemixChange = false
	args.IsSignAction = false
	args.IncludeAllSourceTransactions = false

	result, err := t.activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)
	require.NoError(t, err)

	signedTx = t.buildAndSignTxFromCreateAction(result, parentTx)
	require.NotNil(t, signedTx)

	return result, signedTx
}

func (t *txGeneratorFixture) buildAndSignTxFromCreateAction(createActionResult *wdk.StorageCreateActionResult, _ *transaction.Transaction) *transaction.Transaction {
	t.Helper()
	keyDeriver := sdk.NewKeyDeriver(t.sender.PrivateKey(t))

	signed, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, createActionResult).Assemble()
	require.NoError(t, err)

	err = signed.Sign()
	require.NoError(t, err)

	return signed.Transaction
}

func (t *txGeneratorFixture) Processed() (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction) {
	t.Helper()
	createActionResult, signedTx = t.Created()
	txID := signedTx.TxID().String()

	if t.failedBroadcast {
		t.parent.Provider().ARC().WhenQueryingTx(txID).WillReturnNoBody()
	}

	err := t.performProcess(signedTx, createActionResult.Reference)
	require.NoError(t, err)

	return createActionResult, signedTx
}

func (t *txGeneratorFixture) performProcess(signedTx *transaction.Transaction, reference string) error {
	txID := signedTx.TxID().String()
	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	}

	_, err := t.activeStorage.ProcessAction(t.Context(), t.sender.AuthID(), args)
	return err
}

func (t *txGeneratorFixture) Unprocessed() (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction) {
	t.Helper()

	createActionResult, signedTx = t.Created()

	t.parent.Provider().ScriptsVerifier().WillReturnError(fmt.Errorf("mock scripts verifier error"))
	defer func() {
		t.parent.Provider().ScriptsVerifier().DefaultBehavior()
	}()

	err := t.performProcess(signedTx, createActionResult.Reference)
	require.Errorf(t, err, "expected process to fail due to beef verifier error")

	return createActionResult, signedTx
}

package testabilities

import (
	"fmt"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
	"testing"
)

type TxGeneratorFixture interface {
	Internalized() (internalizeArgs *wdk.InternalizeActionResult, internalizedTx *transaction.Transaction)
	Created() (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction)
	Processed() (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction)
}

func (s *storageFixture) Action(activeStorage *storage.Provider, satoshisOwnedByAlice, satoshisForBob uint64) TxGeneratorFixture {
	return &txGeneratorFixture{
		TB:                    s.t,
		satoshisToInternalize: satoshisOwnedByAlice,
		satoshisToSend:        satoshisForBob,
		parent:                s,
		activeStorage:         activeStorage,
		sender:                testusers.Alice,
		recipient:             testusers.Bob,
	}
}

type txGeneratorFixture struct {
	testing.TB
	parent                *storageFixture
	satoshisToInternalize uint64
	satoshisToSend        uint64
	activeStorage         *storage.Provider
	sender                testusers.User
	recipient             testusers.User
}

func (t *txGeneratorFixture) Internalized() (internalizeResult *wdk.InternalizeActionResult, internalizedTx *transaction.Transaction) {
	t.Helper()
	keyID := brc29.KeyID{
		DerivationPrefix: fixtures.DerivationPrefix,
		DerivationSuffix: fixtures.DerivationSuffix,
	}
	address, err := brc29.Address(fixtures.AnyoneIdentityKey, keyID, t.sender.PublicKey(t))
	require.NoError(t.TB, err)

	lockingScript, err := p2pkh.Lock(address)
	require.NoError(t.TB, err)

	spec := testvectors.GivenTX().
		WithInput(t.satoshisToInternalize+1).
		WithOutputScript(t.satoshisToInternalize, lockingScript)

	internalizeArgs := wdk.InternalizeActionArgs{
		Tx: spec.AtomicBEEF().Bytes(),
		Outputs: []*wdk.InternalizeOutput{
			{
				OutputIndex: 0,
				Protocol:    wdk.WalletPaymentProtocol,
				PaymentRemittance: &wdk.WalletPayment{
					DerivationPrefix:  fixtures.DerivationPrefix,
					DerivationSuffix:  fixtures.DerivationSuffix,
					SenderIdentityKey: fixtures.AnyoneIdentityKey,
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

	result, err := t.activeStorage.InternalizeAction(t.Context(), t.sender.AuthID(), internalizeArgs)
	require.NoError(t, err)

	return result, spec.TX()
}

func (t *txGeneratorFixture) Created() (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction) {
	t.Helper()
	_, parentTx := t.Internalized()

	keyID := brc29.KeyID{
		DerivationPrefix: fixtures.DerivationPrefix,
		DerivationSuffix: fixtures.DerivationSuffix,
	}
	address, err := brc29.Address(t.sender.PrivateKey(t), keyID, t.recipient.PublicKey(t))
	require.NoError(t.TB, err)

	lockingScript, err := p2pkh.Lock(address)
	require.NoError(t.TB, err)

	args := wdk.ValidCreateActionArgs{
		Description: "outputBRC29",
		Inputs: []wdk.ValidCreateActionInput{
			{
				Outpoint: wdk.OutPoint{
					TxID: parentTx.TxID().String(),
					Vout: 0,
				},
				InputDescription:      "provided by previously internalized transaction",
				UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
			},
		},
		Outputs: []wdk.ValidCreateActionOutput{
			{
				LockingScript:      primitives.HexString(lockingScript.String()),
				Satoshis:           primitives.SatoshiValue(t.satoshisToSend),
				OutputDescription:  "output sent to Bob",
				CustomInstructions: to.Ptr(fmt.Sprintf(`{"derivationPrefix":"%s","derivationSuffix":"%s","type":"BRC29"}`, fixtures.DerivationPrefix, fixtures.DerivationSuffix)),
				Tags:               []primitives.StringUnder300{fixtures.CreateActionTestTag},
			},
		},
		LockTime: 0,
		Version:  1,
		Labels:   []primitives.StringUnder300{fixtures.CreateActionTestLabel},
		Options: wdk.ValidCreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr[primitives.BooleanDefaultTrue](false),
			SendWith:               []primitives.TXIDHexString{},
			SignAndProcess:         to.Ptr(primitives.BooleanDefaultTrue(true)),
			KnownTxids:             []primitives.TXIDHexString{},
			NoSendChange:           []wdk.OutPoint{},
			RandomizeOutputs:       false,
			TrustSelf:              to.Ptr(sdk.TrustSelfKnown),
		},
		IsSendWith:                   false,
		IsDelayed:                    false,
		IsNoSend:                     false,
		IsNewTx:                      true,
		IsRemixChange:                false,
		IsSignAction:                 false,
		IncludeAllSourceTransactions: false,
	}

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

func (t *txGeneratorFixture) buildAndSignTxFromCreateAction(createActionResult *wdk.StorageCreateActionResult, parentTx *transaction.Transaction) *transaction.Transaction {
	t.Helper()
	keyDeriver := sdk.NewKeyDeriver(t.sender.PrivateKey(t))

	// FIXME: Workaround START
	// FIXME: Workaround for the fact that the go-sdk's P2PKH Unlocker can't unlock UTXO based on sourceSatoshis & sourceLockingScript
	// FIXME: For now it requires the whole parent transaction to be set in the input
	// FIXME: It should work after this issue is resolved and applied to this project:
	// FIXME: https://github.com/bsv-blockchain/go-sdk/issues/218
	createActionResult.Inputs[0].SourceTransaction = parentTx.Bytes()
	// FIXME: END

	signed, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, createActionResult).Assemble()
	require.NoError(t, err)

	err = signed.Sign()
	require.NoError(t, err)

	return signed
}

func (t *txGeneratorFixture) Processed() (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction) {
	t.Helper()
	createActionResult, signedTx = t.Created()
	txID := signedTx.TxID().String()

	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	}

	_, err := t.activeStorage.ProcessAction(t.Context(), t.sender.AuthID(), args)
	require.NoError(t, err)
	return createActionResult, signedTx
}

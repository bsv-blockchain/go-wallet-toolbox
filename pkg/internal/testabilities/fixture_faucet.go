package testabilities

import (
	"fmt"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testhelper"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	TestBlockHash = "0000000014209ae688e547a58db514ac75e3a10a81ac25b3d357fa92a8ce5128"
)

type faucetFixture struct {
	t          testing.TB
	user       testusers.User
	db         *database.Database
	basketName string
	index      int
}

func (f *faucetFixture) TopUp(satoshis satoshi.Value, opts ...TopUpOpts) (txtestabilities.TransactionSpec, *models.UserUTXO) {
	f.t.Helper()

	options := to.OptionsWithDefault(TopUpOptions{
		Purpose: "test-faucet-purpose",
	}, opts...)

	senderPriv, senderPub := sdk.AnyoneKey()

	_, derivationPrefixBase64 := testhelper.DerivationByNumber(int64(f.index))
	_, derivationSuffixBase64 := testhelper.DerivationByNumber(int64(f.index))

	keyID := brc29.KeyID{
		DerivationPrefix: derivationPrefixBase64,
		DerivationSuffix: derivationSuffixBase64,
	}

	recipientPubKey := f.user.PubKey(f.t)

	lockingScript, err := brc29.LockForCounterparty(senderPriv, keyID, brc29.PubHex(recipientPubKey))
	require.NoError(f.t, err, "Failed to create locking script for top up")

	spec := txtestabilities.GivenTX().
		WithInput(satoshi.MustAdd(satoshis, 1).MustUInt64()).
		WithOutputScript(satoshis.MustUInt64(), lockingScript).
		WithOPReturn(fmt.Sprintf("faucet index %d", f.index))

	txObj := spec.TX()
	if options.Mined {
		txObj.MerklePath = to.Ptr(testutils.MockValidMerklePath(f.t, spec.ID().String(), 1000+uint32(f.index))) //nolint:gosec // test fixture, f.index is always small
	}

	beef, err := txObj.BEEF()
	require.NoError(f.t, err)

	knownTx := &models.KnownTx{
		TxID:      spec.ID().String(),
		Status:    wdk.ProvenTxStatusUnmined,
		RawTx:     spec.TX().Bytes(),
		InputBeef: beef,
	}

	transaction := &models.Transaction{
		UserID:      f.user.ID,
		Status:      wdk.TxStatusUnproven,
		Reference:   fixtures.FaucetReference(spec.ID().String()),
		IsOutgoing:  false,
		Satoshis:    satoshis.Int64(),
		Description: "test-faucet-tx",
		Version:     1,
		LockTime:    0,
		InputBeef:   nil,
		TxID:        to.Ptr(spec.ID().String()),
	}

	if len(options.Labels) > 0 {
		transaction.Labels = slices.Map(options.Labels, func(label string) *models.Label {
			return &models.Label{
				Name:   label,
				UserID: f.user.ID,
			}
		})
	}

	output := &models.Output{
		Vout:              0,
		UserID:            f.user.ID,
		Satoshis:          satoshis.Int64(),
		Spendable:         true,
		Change:            true,
		ProvidedBy:        string(wdk.ProvidedByStorage),
		Description:       "test-faucet-output",
		Purpose:           options.Purpose,
		Type:              string(wdk.OutputTypeP2PKH),
		DerivationPrefix:  to.Ptr(derivationPrefixBase64),
		DerivationSuffix:  to.Ptr(derivationSuffixBase64),
		LockingScript:     spec.TX().Outputs[0].LockingScript.Bytes(),
		BasketName:        &f.basketName,
		SenderIdentityKey: to.Ptr(senderPub.ToDERHex()),

		Transaction: transaction,

		Tags: []*models.Tag{
			{
				Name:   fixtures.CreateActionTestTag,
				UserID: f.user.ID,
			},
			{
				Name:   fixtures.FaucetTag(f.index),
				UserID: f.user.ID,
			},
		},
	}

	utxo := &models.UserUTXO{
		UserID:             f.user.ID,
		Satoshis:           satoshis.MustUInt64(),
		EstimatedInputSize: txutils.P2PKHEstimatedInputSize,
		BasketName:         f.basketName,
		UTXOStatus:         wdk.UTXOStatusUnproven,

		Output: output,
	}

	if txObj.MerklePath != nil {
		merkleRoot, err := txObj.MerklePath.ComputeRootHex(to.Ptr(spec.ID().String()))
		require.NoError(f.t, err)

		knownTx.Status = wdk.ProvenTxStatusCompleted
		knownTx.BlockHeight = &txObj.MerklePath.BlockHeight
		knownTx.MerklePath = txObj.MerklePath.Bytes()
		knownTx.MerkleRoot = to.Ptr(merkleRoot)
		knownTx.BlockHash = to.Ptr(TestBlockHash)

		transaction.Status = wdk.TxStatusCompleted
	}

	tx := f.db.DB.WithContext(f.t.Context())
	tx.Create(utxo)
	tx.Create(knownTx)

	f.index++

	return spec, utxo
}

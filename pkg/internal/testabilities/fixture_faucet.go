package testabilities

import (
	"fmt"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testhelper"
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

func (f *faucetFixture) TopUp(satoshis satoshi.Value, opts ...TopUpOpts) (txtestabilities.TransactionSpec, *models.Output) {
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

	knownTx := &models.ProvenTxReq{
		TxID:         spec.ID().String(),
		Status:       wdk.ProvenTxStatusUnmined,
		WasBroadcast: true,
		RawTx:        spec.TX().Bytes(),
		InputBeef:    beef,
	}

	transaction := &models.Transaction{
		UserID:      f.user.ID,
		Status:      wdk.TxStatusUnproven,
		Reference:   fixtures.FaucetReference(spec.ID().String()),
		IsOutgoing:  false,
		Satoshis:    satoshis.Int64(),
		Description: "test-faucet-tx",
		Version:     to.Ptr[uint32](1),
		LockTime:    to.Ptr[uint32](0),
		InputBeef:   nil,
		TxID:        to.Ptr(spec.ID().String()),
	}

	if len(options.Labels) > 0 {
		transaction.Labels = slices.Map(options.Labels, func(label string) *models.TxLabel {
			return &models.TxLabel{
				Label:  label,
				UserID: f.user.ID,
			}
		})
	}

	var basket models.OutputBasket
	err = f.db.DB.Where("name = ? AND userId = ?", f.basketName, f.user.ID).First(&basket).Error
	require.NoError(f.t, err, "Failed to find basket in faucet")
	f.t.Logf("FETCHED BASKET: %+v, BasketID=%d", basket, basket.BasketID)

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
		BasketID:          to.Ptr(basket.BasketID),
		SenderIdentityKey: to.Ptr(senderPub.ToDERHex()),
		Txid:              to.Ptr(spec.ID().String()),

		Tags: []*models.OutputTag{
			{
				Tag:    fixtures.CreateActionTestTag,
				UserID: f.user.ID,
			},
			{
				Tag:    fixtures.FaucetTag(f.index),
				UserID: f.user.ID,
			},
		},
	}

	var provenTx *models.ProvenTx
	if txObj.MerklePath != nil {
		merkleRoot, err := txObj.MerklePath.ComputeRootHex(to.Ptr(spec.ID().String()))
		require.NoError(f.t, err)

		knownTx.Status = wdk.ProvenTxStatusCompleted
		provenTx = &models.ProvenTx{
			TxID:       spec.ID().String(),
			Height:     to.Ptr(txObj.MerklePath.BlockHeight),
			MerklePath: txObj.MerklePath.Bytes(),
			MerkleRoot: to.Ptr(merkleRoot),
			BlockHash:  to.Ptr(TestBlockHash),
			RawTx:      spec.TX().Bytes(),
		}

		transaction.Status = wdk.TxStatusCompleted
	}

	tx := f.db.DB.WithContext(f.t.Context())

	// We insert transaction first
	err = tx.Create(transaction).Error
	require.NoError(f.t, err)

	linkFaucetTxLabels(f.t, tx, transaction.TransactionID, transaction.Labels)

	// Set the transaction ID on the output
	output.TransactionID = transaction.TransactionID

	// Insert the output separately without omitting associations, as output.Basket is already nil
	err = tx.Create(output).Error
	require.NoError(f.t, err, "Failed to create faucet output")

	linkFaucetOutputTags(f.t, tx, output.OutputID, output.Tags)

	if provenTx != nil {
		err = tx.Create(provenTx).Error
		require.NoError(f.t, err, "Failed to create faucet proven tx")
		knownTx.ProvenTxID = &provenTx.ProvenTxID

		err = tx.Model(&models.Transaction{}).
			Where("transactionId = ?", transaction.TransactionID).
			Update("provenTxId", provenTx.ProvenTxID).Error
		require.NoError(f.t, err, "Failed to link faucet transaction to proven tx")
	}

	err = tx.Create(knownTx).Error
	require.NoError(f.t, err, "Failed to create faucet known tx")

	f.index++

	return spec, output
}

// linkFaucetOutputTags upserts each tag (by tag+userId) and links it to outputID via
// bsv_output_tags_map. Tags are no longer a GORM-managed association (see models.Output.Tags),
// so callers that create outputs directly (bypassing repo.Transactions) must link tags themselves.
func linkFaucetOutputTags(t testing.TB, tx *gorm.DB, outputID uint, tags []*models.OutputTag) {
	t.Helper()

	for _, tag := range tags {
		tagModel := &models.OutputTag{Tag: tag.Tag, UserID: tag.UserID}
		err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(tagModel).Error
		require.NoError(t, err, "Failed to upsert output tag")

		if tagModel.OutputTagID == 0 {
			err = tx.Where("tag = ? AND userId = ?", tag.Tag, tag.UserID).First(tagModel).Error
			require.NoError(t, err, "Failed to find existing output tag")
		}

		err = tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.OutputTagsMap{OutputID: outputID, OutputTagID: tagModel.OutputTagID}).Error
		require.NoError(t, err, "Failed to link output tag")
	}
}

// linkFaucetTxLabels upserts each label (by label+userId) and links it to transactionID via
// bsv_tx_labels_map. Labels are no longer a GORM-managed association (see models.Transaction.Labels),
// so callers that create transactions directly (bypassing repo.Transactions) must link labels themselves.
func linkFaucetTxLabels(t testing.TB, tx *gorm.DB, transactionID uint, labels []*models.TxLabel) {
	t.Helper()

	for _, label := range labels {
		labelModel := &models.TxLabel{Label: label.Label, UserID: label.UserID}
		err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(labelModel).Error
		require.NoError(t, err, "Failed to upsert tx label")

		if labelModel.TxLabelID == 0 {
			err = tx.Where("label = ? AND userId = ?", label.Label, label.UserID).First(labelModel).Error
			require.NoError(t, err, "Failed to find existing tx label")
		}

		err = tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.TxLabelsMap{TxLabelID: labelModel.TxLabelID, TransactionID: transactionID}).Error
		require.NoError(t, err, "Failed to link tx label")
	}
}

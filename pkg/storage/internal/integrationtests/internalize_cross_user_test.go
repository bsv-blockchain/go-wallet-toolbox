package integrationtests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/tsgenerated"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestInternalizeCrossUser(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// The beef we will use for internalization
	beef := tsgenerated.ParentTransactionAtomicBeef(t)
	txID := "756754d5ad8f00e05c36d89a852971c0a1dc0c10f20cd7840ead347aff475ef6" // txID of the beef

	t.Run("User A's transaction is waiting to broadcast", func(t *testing.T) {
		// Simulate User A having this transaction in "sending" state
		knownTx := models.KnownTx{
			TxID:   txID,
			Status: wdk.ProvenTxStatusSending,
			RawTx:  []byte("fake_raw_tx"),
		}
		err := activeStorage.Database.DB.Create(&knownTx).Error
		require.NoError(t, err)

		// Verify it's sending
		var check models.KnownTx
		err = activeStorage.Database.DB.First(&check, "tx_id = ?", txID).Error
		require.NoError(t, err)
		assert.Equal(t, wdk.ProvenTxStatusSending, check.Status)
	})

	t.Run("User B internalizes same transaction", func(t *testing.T) {
		// User B receives the transaction and internalizes it
		internalizeArgs := wdk.InternalizeActionArgs{
			Tx: beef,
			Outputs: []*wdk.InternalizeOutput{
				{
					OutputIndex: 0,
					Protocol:    wdk.WalletPaymentProtocol,
					PaymentRemittance: &wdk.WalletPayment{
						DerivationPrefix:  derPrefix,
						DerivationSuffix:  derSuffix,
						SenderIdentityKey: primitives.PubKeyHex(testusers.Bob.IdentityKey(t)),
					},
				},
			},
			Labels:         []primitives.StringUnder300{"userB_label"},
			Description:    "internalized from User A",
			SeekPermission: nil,
		}

		given.Provider().BHS().OnMerkleRootVerifyResponse(
			tsgenerated.BeefToInternalizeHeight,
			tsgenerated.BeefToInternalizeMerkleRoot,
			testabilities.BHSMerkleRootConfirmed,
		)

		_, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Bob.AuthID(),
			internalizeArgs,
		)
		require.NoError(t, err)

		// Check the global KnownTx status again
		var check models.KnownTx
		err = activeStorage.Database.DB.First(&check, "tx_id = ?", txID).Error
		require.NoError(t, err)

		assert.Equal(t, wdk.ProvenTxStatusSending, check.Status, "KnownTx was correctly preserved as Sending")
	})
}

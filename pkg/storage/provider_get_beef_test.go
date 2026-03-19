package storage_test

import (
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestGetBeef(t *testing.T) {
	t.Run("empty storage, fetched from services, the tx has merkle path", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		givenMinedTx := given.Provider().WhatsOnChain().MinedTransaction()
		givenMinedTx.WillReturnRawTx()
		givenMinedTx.WillReturnMerklePath()
		txID := givenMinedTx.TxID()

		// and:
		activeStorage := given.Provider().GORM()

		// when:
		beef, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{})

		// then:
		require.NoError(t, err)
		require.NotNil(t, beef)

		assert.NotNil(t, beef.FindTransaction(txID))
	})

	t.Run("storage has parent transaction (mined), child tx needs to be fetched from services", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := given.Provider().GORM()

		parentTx := given.Provider().WhatsOnChain().MinedTransaction().Tx()

		atomicBeef, err := parentTx.AtomicBEEF(false)
		require.NoError(t, err)

		given.Provider().BHS().OnMerkleRootVerifyResponse(
			1687775,
			"6861d579c2fb885c2fef10ce39c2750d9b50c4185727b19989de657fa105d1b7",
			testabilities.BHSMerkleRootConfirmed,
		)

		args := fixtures.DefaultInternalizeActionArgs(t, wdk.BasketInsertionProtocol)
		args.Tx = atomicBeef

		_, err = activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)
		require.NoError(t, err)

		// and:
		childTxSpec := testvectors.GivenTX().WithInputFromUTXO(parentTx, 0).WithP2PKHOutput(1)
		childTxID := childTxSpec.ID().String()
		given.Provider().WhatsOnChain().WillRespondWithRawTx(200, childTxID, childTxSpec.RawTX().Hex(), nil)
		given.Provider().WhatsOnChain().WillRespondWithMerklePath(404, childTxID, "")

		// when:
		beef, err := activeStorage.GetBeefForTransaction(t.Context(), childTxID, wdk.StorageGetBeefOptions{})

		// then:
		require.NoError(t, err)
		require.NotNil(t, beef)

		assert.NotNil(t, beef.FindTransaction(childTxID))
		assert.NotNil(t, beef.FindTransaction(parentTx.TxID().String()))
	})

	t.Run("ignoreStorage option, fetched from services, the tx has merkle path", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		givenMinedTx := given.Provider().WhatsOnChain().MinedTransaction()
		givenMinedTx.WillReturnRawTx()
		givenMinedTx.WillReturnMerklePath()
		txID := givenMinedTx.TxID()

		activeStorage := given.Provider().GORM()

		// when:
		beef, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{IgnoreStorage: true})

		// then:
		require.NoError(t, err)
		require.NotNil(t, beef)

		assert.NotNil(t, beef.FindTransaction(txID))
	})

	t.Run("empty storage, fetched from services, should fail, the tx doesn't have merkle path", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		givenMinedTx := given.Provider().WhatsOnChain().MinedTransaction()
		givenMinedTx.WillReturnRawTx()
		txID := givenMinedTx.TxID()

		given.Provider().WhatsOnChain().WillRespondWithMerklePath(404, txID, "")

		activeStorage := given.Provider().GORM()

		// when:
		_, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{})

		// then:
		require.Error(t, err)
	})
}

func TestGetBeef_WithOptions(t *testing.T) {
	t.Run("knownTxIDs target returns txid-only", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		givenMinedTx := given.Provider().WhatsOnChain().MinedTransaction()
		txID := givenMinedTx.TxID()

		activeStorage := given.Provider().GORM()

		// when:
		beef, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{KnownTxIDs: []string{txID}})

		// then:
		require.NoError(t, err)
		require.NotNil(t, beef)
		btx := beef.FindTransaction(txID)
		assert.Nil(t, btx)
	})

	t.Run("trustSelf known represents current tx as txid-only", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		givenMinedTx := given.Provider().WhatsOnChain().MinedTransaction()
		givenMinedTx.WillReturnRawTx()
		givenMinedTx.WillReturnMerklePath()
		txID := givenMinedTx.TxID()

		activeStorage := given.Provider().GORM()

		// and:
		_, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{IgnoreNewProven: false})
		require.NoError(t, err)

		// when:
		beef, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{TrustSelf: sdk.TrustSelfKnown, IgnoreServices: true})

		// then:
		require.NoError(t, err)
		require.NotNil(t, beef)
		btx := beef.FindTransaction(txID)
		assert.Nil(t, btx)
	})

	t.Run("minProofLevel ignores proof at depth 0", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		parent := given.Provider().WhatsOnChain().MinedTransaction()
		parent.WillReturnRawTx()
		parent.WillReturnMerklePath()

		childSpec := testvectors.GivenTX().WithInputFromUTXO(parent.Tx(), 0).WithP2PKHOutput(1)
		childTxID := childSpec.ID().String()
		given.Provider().WhatsOnChain().WillRespondWithRawTx(200, childTxID, childSpec.RawTX().Hex(), nil)
		given.Provider().WhatsOnChain().WillRespondWithMerklePath(404, childTxID, "")

		activeStorage := given.Provider().GORM()

		// when:
		beef, err := activeStorage.GetBeefForTransaction(t.Context(), childTxID, wdk.StorageGetBeefOptions{MinProofLevel: 1})

		// then:
		require.NoError(t, err)
		require.NotNil(t, beef)
		btx := beef.FindTransaction(childTxID)
		require.NotNil(t, btx)
		assert.Nil(t, btx.MerklePath)
		assert.NotEmpty(t, btx.Hex())

		// and:
		ptx := beef.FindTransaction(parent.TxID())
		require.NotNil(t, ptx)
		assert.NotNil(t, ptx.MerklePath)
	})

	t.Run("knownTxIDs on inputs merges parent as txid-only", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		parent := given.Provider().WhatsOnChain().MinedTransaction().Tx()

		childSpec := testvectors.GivenTX().WithInputFromUTXO(parent, 0).WithP2PKHOutput(1)
		childTxID := childSpec.ID().String()
		given.Provider().WhatsOnChain().WillRespondWithRawTx(200, childTxID, childSpec.RawTX().Hex(), nil)
		given.Provider().WhatsOnChain().WillRespondWithMerklePath(404, childTxID, "")

		activeStorage := given.Provider().GORM()

		// when:
		beef, err := activeStorage.GetBeefForTransaction(t.Context(), childTxID, wdk.StorageGetBeefOptions{KnownTxIDs: []string{parent.TxID().String()}})

		// then:
		require.NoError(t, err)
		require.NotNil(t, beef)
		child := beef.FindTransaction(childTxID)
		require.NotNil(t, child)
		p := beef.FindTransaction(parent.TxID().String())
		assert.Nil(t, p)
	})
}

func TestGetBeef_PersistNewProven_WhenIgnoreNewProvenFalse(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	mined := given.Provider().WhatsOnChain().MinedTransaction()
	mined.WillReturnRawTx()
	mined.WillReturnMerklePath()
	txID := mined.TxID()

	activeStorage := given.Provider().GORM()

	// when:
	beef, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{IgnoreNewProven: false})

	// then:
	require.NoError(t, err)
	require.NotNil(t, beef)
	assert.NotNil(t, beef.FindTransaction(txID))

	// and when:
	beef2, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{IgnoreServices: true})

	// then:
	require.NoError(t, err)
	require.NotNil(t, beef2)
	assert.NotNil(t, beef2.FindTransaction(txID))
}

func TestGetBeef_IgnoreNewProven_DoesNotPersist(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	mined := given.Provider().WhatsOnChain().MinedTransaction()
	mined.WillReturnRawTx()
	mined.WillReturnMerklePath()
	txID := mined.TxID()

	activeStorage := given.Provider().GORM()

	// when:
	beef, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{IgnoreNewProven: true})

	// then:
	require.NoError(t, err)
	require.NotNil(t, beef)
	assert.NotNil(t, beef.FindTransaction(txID))

	// and when:
	_, err = activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{IgnoreServices: true})

	// then:
	require.Error(t, err)
	require.ErrorContains(t, err, "not known to storage")
}

func TestGetBeef_IgnoreServices_WithMissingStorage_ReturnsError(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	mined := given.Provider().WhatsOnChain().MinedTransaction()
	txID := mined.TxID()

	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{IgnoreServices: true})

	// then:
	require.Error(t, err)
	require.ErrorContains(t, err, "not known to storage")
}

func TestGetBeef_ServicesPath_WithKnownTxIDsContainsTarget_ReturnsTxIDOnly(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	mined := given.Provider().WhatsOnChain().MinedTransaction()
	mined.WillReturnRawTx()
	mined.WillReturnMerklePath()
	txID := mined.TxID()

	activeStorage := given.Provider().GORM()

	// when:
	beef, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{IgnoreStorage: true, KnownTxIDs: []string{txID}})

	// then:
	require.NoError(t, err)
	require.NotNil(t, beef)
	assert.Nil(t, beef.FindTransaction(txID))
}

func TestGetBeef_KnownTxIDsWithDuplicates_OnInputs_MergesParentAsTxIDOnly(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	parent := given.Provider().WhatsOnChain().MinedTransaction().Tx()

	childSpec := testvectors.GivenTX().WithInputFromUTXO(parent, 0).WithP2PKHOutput(1)
	childTxID := childSpec.ID().String()
	given.Provider().WhatsOnChain().WillRespondWithRawTx(200, childTxID, childSpec.RawTX().Hex(), nil)
	given.Provider().WhatsOnChain().WillRespondWithMerklePath(404, childTxID, "")

	activeStorage := given.Provider().GORM()

	// when:
	dupParent := parent.TxID().String()
	beef, err := activeStorage.GetBeefForTransaction(t.Context(), childTxID, wdk.StorageGetBeefOptions{KnownTxIDs: []string{dupParent, dupParent, dupParent}})

	// then:
	require.NoError(t, err)
	require.NotNil(t, beef)
	child := beef.FindTransaction(childTxID)
	require.NotNil(t, child)
	p := beef.FindTransaction(parent.TxID().String())
	assert.Nil(t, p)
}

func TestGetBeef_ServicesRawTxError_Propagates(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	parent := given.Provider().WhatsOnChain().MinedTransaction().Tx()
	childSpec := testvectors.GivenTX().WithInputFromUTXO(parent, 0).WithP2PKHOutput(1)
	childTxID := childSpec.ID().String()

	given.Provider().WhatsOnChain().WillRespondWithRawTx(500, childTxID, "", nil)

	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.GetBeefForTransaction(t.Context(), childTxID, wdk.StorageGetBeefOptions{})

	// then:
	require.Error(t, err)
}

func TestGetBeef_ServicesReturnsNilRawTx_ReturnsError(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	parent := given.Provider().WhatsOnChain().MinedTransaction().Tx()
	childSpec := testvectors.GivenTX().WithInputFromUTXO(parent, 0).WithP2PKHOutput(1)
	childTxID := childSpec.ID().String()

	given.Provider().WhatsOnChain().WillRespondWithRawTx(200, childTxID, "", nil)

	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.GetBeefForTransaction(t.Context(), childTxID, wdk.StorageGetBeefOptions{})

	// then:
	require.Error(t, err)
}

func TestGetBeef_ServicesMerklePathError_Propagates(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	parent := given.Provider().WhatsOnChain().MinedTransaction().Tx()
	childSpec := testvectors.GivenTX().WithInputFromUTXO(parent, 0).WithP2PKHOutput(1)
	childTxID := childSpec.ID().String()

	given.Provider().WhatsOnChain().WillRespondWithRawTx(200, childTxID, childSpec.RawTX().Hex(), nil)
	given.Provider().WhatsOnChain().WillRespondWithMerklePath(500, childTxID, "")

	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.GetBeefForTransaction(t.Context(), childTxID, wdk.StorageGetBeefOptions{})

	// then:
	require.Error(t, err)
}

func TestGetBeef_ServicesReturnsInvalidRawTx_ReturnsError(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	parent := given.Provider().WhatsOnChain().MinedTransaction().Tx()
	childSpec := testvectors.GivenTX().WithInputFromUTXO(parent, 0).WithP2PKHOutput(1)
	childTxID := childSpec.ID().String()

	given.Provider().WhatsOnChain().WillRespondWithRawTx(200, childTxID, "00", nil)
	given.Provider().WhatsOnChain().WillRespondWithMerklePath(404, childTxID, "")

	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.GetBeefForTransaction(t.Context(), childTxID, wdk.StorageGetBeefOptions{})

	// then:
	require.Error(t, err)
}

func TestGetBeef_ServicesReturnsTxWithMissingSourceTXID_ReturnsError(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	spec := testvectors.GivenTX().WithInput(1).WithP2PKHOutput(1)
	txID := spec.ID().String()
	given.Provider().WhatsOnChain().WillRespondWithRawTx(200, txID, spec.RawTX().Hex(), nil)
	given.Provider().WhatsOnChain().WillRespondWithMerklePath(404, txID, "")

	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{})

	// then:
	require.Error(t, err)
}

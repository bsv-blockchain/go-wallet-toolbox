package testabilities

import (
	"fmt"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

const (
	MockReference        = "mock-reference"
	MockDerivationPrefix = "mock-derivation-prefix"
	MockDerivationSuffix = "mock-derivation-suffix"
	TestBlockHash        = "0000000014209ae688e547a58db514ac75e3a10a81ac25b3d357fa92a8ce5128"
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
	options := TopUpOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	spec := txtestabilities.GivenTX().
		WithInput(satoshi.MustAdd(satoshis, 1).MustUInt64()).
		WithP2PKHOutput(satoshis.MustUInt64()).
		WithOPReturn(fmt.Sprintf("faucet index %d", f.index))

	txObj := spec.TX()
	if options.Mined {
		txObj.MerklePath = mockValidMerklePath(f.t, spec.ID())
	}

	beef, err := txObj.BEEF()
	require.NoError(f.t, err)

	provenTxReq := &models.ProvenTxReq{
		TxID:      spec.ID(),
		Status:    wdk.ProvenTxStatusUnmined,
		RawTx:     spec.TX().Bytes(),
		InputBeef: beef,
	}

	if txObj.MerklePath != nil {
		merkleRoot, err := txObj.MerklePath.ComputeRootHex(to.Ptr(spec.ID()))
		require.NoError(f.t, err)

		provenTxReq.Status = wdk.ProvenTxStatusCompleted
		provenTxReq.BlockHeight = &txObj.MerklePath.BlockHeight
		provenTxReq.MerklePath = txObj.MerklePath.Bytes()
		provenTxReq.MerkleRoot = to.Ptr(merkleRoot)
		provenTxReq.BlockHash = to.Ptr(TestBlockHash)
	}

	transaction := &models.Transaction{
		UserID:      f.user.ID,
		Status:      wdk.TxStatusCompleted,
		Reference:   MockReference,
		IsOutgoing:  false,
		Satoshis:    satoshis.Int64(),
		Description: "test-faucet-tx",
		Version:     1,
		LockTime:    0,
		InputBeef:   nil,
		TxID:        to.Ptr(spec.ID()),
	}

	output := &models.Output{
		Vout:             0,
		UserID:           f.user.ID,
		Satoshis:         satoshis.Int64(),
		Spendable:        true,
		Change:           true,
		ProvidedBy:       string(wdk.ProvidedByStorage),
		Description:      "test-faucet-output",
		Purpose:          "test-faucet-purpose",
		Type:             string(wdk.OutputTypeP2PKH),
		DerivationPrefix: to.Ptr(fmt.Sprintf("%s/%d", MockDerivationPrefix, f.index)),
		DerivationSuffix: to.Ptr(fmt.Sprintf("%s/%d", MockDerivationSuffix, f.index)),
		LockingScript:    to.Ptr(spec.TX().Outputs[0].LockingScript.String()),
		BasketName:       &f.basketName,

		Transaction: transaction,
	}

	utxo := &models.UserUTXO{
		UserID:             f.user.ID,
		Satoshis:           satoshis.MustUInt64(),
		EstimatedInputSize: txutils.P2PKHEstimatedInputSize,
		BasketName:         f.basketName,

		Output: output,
	}

	tx := f.db.DB.WithContext(f.t.Context())
	tx.Create(utxo)
	tx.Create(provenTxReq)

	return spec, utxo
}

func mockValidMerklePath(t testing.TB, txID string) *sdk.MerklePath {
	t.Helper()

	hash, err := chainhash.NewHashFromHex(txID)
	require.NoError(t, err)

	someSecondHash, errHash := chainhash.NewHashFromHex("27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e")
	require.NoError(t, errHash)

	return &sdk.MerklePath{
		BlockHeight: 2000,
		Path: [][]*sdk.PathElement{
			{
				{
					Offset: 0,
					Hash:   hash,
					Txid:   to.Ptr(true),
				},
				{
					Offset: 1,
					Hash:   someSecondHash,
				},
			},
		},
	}
}

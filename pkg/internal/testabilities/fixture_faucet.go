package testabilities

import (
	"fmt"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

const (
	MockReference        = "mock-reference"
	MockDerivationPrefix = "mock-derivation-prefix"
	MockDerivationSuffix = "mock-derivation-suffix"
)

type faucetFixture struct {
	t          testing.TB
	user       testusers.User
	db         *database.Database
	basketName string
	index      int
}

func (f *faucetFixture) TopUp(satoshis satoshi.Value) (txtestabilities.TransactionSpec, *models.UserUTXO) {
	f.t.Helper()

	spec := txtestabilities.GivenTX().
		WithInput(satoshi.MustAdd(satoshis, 1).MustUInt64()).
		WithP2PKHOutput(satoshis.MustUInt64()).
		WithOPReturn(fmt.Sprintf("faucet index %d", f.index))

	beef, err := spec.TX().BEEF()
	require.NoError(f.t, err)

	provenTxReq := &models.ProvenTxReq{
		TxID:      spec.ID(),
		Status:    wdk.ProvenTxStatusUnmined,
		RawTx:     spec.TX().Bytes(),
		InputBeef: beef,
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

		Output: output,
	}

	tx := f.db.DB.WithContext(f.t.Context())
	tx.Create(utxo)
	tx.Create(provenTxReq)

	f.index++

	return spec, utxo
}

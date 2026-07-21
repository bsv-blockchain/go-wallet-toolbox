package funder_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestSpendTiers(t *testing.T) {
	assert.Equal(t, []wdk.UTXOStatus{wdk.UTXOStatusMined}, funder.SpendTiers(defs.SpendPolicyMinedOnly))
	assert.Equal(t, []wdk.UTXOStatus{wdk.UTXOStatusMined, wdk.UTXOStatusUnproven}, funder.SpendTiers(defs.SpendPolicyPreferMined))
	assert.Equal(t, []wdk.UTXOStatus{wdk.UTXOStatusMined, wdk.UTXOStatusUnproven, wdk.UTXOStatusSending}, funder.SpendTiers(defs.SpendPolicyAny))
	assert.Equal(t, funder.SpendTiers(defs.SpendPolicyPreferMined), funder.SpendTiers("unknown"), "unknown policy defaults to prefer_mined")
}

func baseFundArgs(basket *entity.OutputBasket, userID int, constraints funder.Constraints, tx *gorm.DB) funder.FundArgs {
	return funder.FundArgs{
		TargetSat:     1000,
		CurrentTxSize: 44,
		OutputCount:   1,
		Basket:        basket,
		UserID:        userID,
		Constraints:   constraints,
		Tx:            tx,
	}
}

func TestFundWithConstraints_TierOverride(t *testing.T) {
	given, _, cleanup := testabilities.New(t)
	defer cleanup()

	// given: a funder and an unproven-only pool
	funderSvc := given.NewFunderService()
	basket := given.BasketFor(testusers.Alice).WithNumberOfDesiredUTXOs(30)
	given.UTXO().OwnedBy(testusers.Alice).InBasket(basket).WithSatoshis(10_000).P2PKH().
		WithStatus(wdk.UTXOStatusUnproven).Stored()

	minedOnly := funder.Constraints{Tiers: funder.SpendTiers(defs.SpendPolicyMinedOnly)}

	// when/then: mined_only tiers cannot claim unproven rows
	tx := given.GormDB().Begin()
	defer tx.Rollback()
	_, err := funderSvc.FundWithConstraints(t.Context(), baseFundArgs(basket, testusers.Alice.ID, minedOnly, tx))
	require.ErrorIs(t, err, wdk.ErrNotEnoughFunds)

	// and: prefer_mined tiers claim them
	preferMined := funder.Constraints{Tiers: funder.SpendTiers(defs.SpendPolicyPreferMined)}
	result, err := funderSvc.FundWithConstraints(t.Context(), baseFundArgs(basket, testusers.Alice.ID, preferMined, tx))
	require.NoError(t, err)
	require.Len(t, result.AllocatedUTXOs, 1)
}

func TestFundWithConstraints_MaxChangeOutputsCap(t *testing.T) {
	given, _, cleanup := testabilities.New(t)
	defer cleanup()

	// given: a large mined UTXO whose change would normally split into many outputs
	funderSvc := given.NewFunderService()
	basket := given.BasketFor(testusers.Alice).WithNumberOfDesiredUTXOs(30)
	given.UTXO().OwnedBy(testusers.Alice).InBasket(basket).WithSatoshis(100_000).P2PKH().
		WithStatus(wdk.UTXOStatusMined).Stored()

	singleChange := funder.Constraints{MaxChangeOutputs: 1}

	// when:
	tx := given.GormDB().Begin()
	defer tx.Rollback()
	result, err := funderSvc.FundWithConstraints(t.Context(), baseFundArgs(basket, testusers.Alice.ID, singleChange, tx))

	// then: exactly one change output regardless of the change amount
	require.NoError(t, err)
	assert.EqualValues(t, 1, result.ChangeOutputsCount)
}

func TestFundWithConstraints_ZeroValueMatchesFund(t *testing.T) {
	given, _, cleanup := testabilities.New(t)
	defer cleanup()

	funderSvc := given.NewFunderService()
	basket := given.BasketFor(testusers.Alice).WithNumberOfDesiredUTXOs(30)
	given.UTXO().OwnedBy(testusers.Alice).InBasket(basket).WithSatoshis(50_000).P2PKH().
		WithStatus(wdk.UTXOStatusMined).Stored()

	tx := given.GormDB().Begin()
	legacy, err := funderSvc.Fund(t.Context(), 1000, 44, 1, basket, testusers.Alice.ID, nil, nil, false, false, 0, tx)
	require.NoError(t, err)
	tx.Rollback()

	tx2 := given.GormDB().Begin()
	defer tx2.Rollback()
	constrained, err := funderSvc.FundWithConstraints(t.Context(), baseFundArgs(basket, testusers.Alice.ID, funder.Constraints{}, tx2))
	require.NoError(t, err)

	assert.Equal(t, legacy.ChangeOutputsCount, constrained.ChangeOutputsCount)
	assert.Equal(t, legacy.ChangeAmount, constrained.ChangeAmount)
	assert.Equal(t, legacy.Fee, constrained.Fee)
	assert.Len(t, constrained.AllocatedUTXOs, len(legacy.AllocatedUTXOs))
}

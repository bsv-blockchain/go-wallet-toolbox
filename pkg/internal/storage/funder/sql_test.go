package funder_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestFunderSQLFund(t *testing.T) {
	const smallTransactionSize = 44
	const transactionSizeForHigherFee = 1001
	const noOutputs = uint64(0)
	const oneOutput = uint64(1)
	ctx := t.Context()

	testCasesErrors := map[string]struct {
		thereAreUTXOInDB func(testabilities.FunderFixture, *entity.OutputBasket)
		targetSatoshis   satoshi.Value
		txSize           uint64
		outputCount      uint64
	}{
		"return error when user has no utxo": {
			thereAreUTXOInDB: func(testabilities.FunderFixture, *entity.OutputBasket) {},

			targetSatoshis: 100,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,
		},
		"return error when user fund the transaction by himself but has not enough utxo to cover the fee": {
			thereAreUTXOInDB: func(testabilities.FunderFixture, *entity.OutputBasket) {},

			targetSatoshis: 0,
			txSize:         smallTransactionSize,
			outputCount:    noOutputs,
		},
		"return error when user has not enough utxo to cover the transaction": {
			thereAreUTXOInDB: func(given testabilities.FunderFixture, basket *entity.OutputBasket) {
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(50).P2PKH().Stored()
			},

			targetSatoshis: 100,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,
		},
		"return error when user has not enough utxos to cover fee": {
			thereAreUTXOInDB: func(given testabilities.FunderFixture, basket *entity.OutputBasket) {
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(100).P2PKH().Stored()
			},

			targetSatoshis: 100,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,
		},
		"return error when user has not enough utxos to cover fee for bigger tx": {
			// Because the transaction size makes the fee = 2, one satoshi above the target satoshis is not enough.
			thereAreUTXOInDB: func(given testabilities.FunderFixture, basket *entity.OutputBasket) {
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(101).P2PKH().Stored()
			},

			targetSatoshis: 100,
			txSize:         transactionSizeForHigherFee,
			outputCount:    oneOutput,
		},
		"return error when user has no utxos but there are other users utxos": {
			thereAreUTXOInDB: func(given testabilities.FunderFixture, basket *entity.OutputBasket) {
				given.UTXO().OwnedBy(testusers.Bob).WithSatoshis(1000).P2PKH().Stored()
				given.UTXO().OwnedBy(testusers.Bob).WithSatoshis(100).P2PKH().Stored()
				given.UTXO().OwnedBy(testusers.Bob).WithSatoshis(200).P2PKH().Stored()
				given.UTXO().OwnedBy(testusers.Bob).WithSatoshis(300).P2PKH().Stored()
			},

			targetSatoshis: 100,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,
		},
		"return error when user has utxos but in other basket": {
			thereAreUTXOInDB: func(given testabilities.FunderFixture, basket *entity.OutputBasket) {
				otherBasket := *basket
				otherBasket.Name = "other_basket"

				given.UTXO().InBasket(&otherBasket).OwnedBy(testusers.Alice).WithSatoshis(10_000).P2PKH().Stored()
				given.UTXO().InBasket(&otherBasket).OwnedBy(testusers.Alice).WithSatoshis(10_000).P2PKH().Stored()
				given.UTXO().InBasket(&otherBasket).OwnedBy(testusers.Alice).WithSatoshis(10_000).P2PKH().Stored()
			},

			targetSatoshis: 100,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,
		},
	}
	for name, test := range testCasesErrors {
		t.Run(name, func(t *testing.T) {
			// given:
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			// and:
			funder := given.NewFunderService()

			// and:
			basket := given.BasketFor(testusers.Alice).ThatPrefersSingleChange()

			// and:
			test.thereAreUTXOInDB(given, basket)

			// when:
			result, err := funder.Fund(ctx, test.targetSatoshis, test.txSize, test.outputCount, basket, testusers.Alice.ID, nil, nil, false, false)

			// then:
			then.Result(result).WithError(err)
		})
	}

	// CreateAction can receive args with inputs that aren't tracked by this wallet
	// those are the test cases for handling such transactions with inputs.
	testCasesForFundingWithoutAllocatingUTXO := map[string]struct {
		possessedUTXOs int64
		targetSatoshis satoshi.Value
		txSize         uint64
		outputCount    uint64
		expectations   func(testabilities.SuccessFundingResultAssertion)
	}{
		"user has funded exactly the transaction and fee by himself": {
			targetSatoshis: -1,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,

			expectations: func(thenResult testabilities.SuccessFundingResultAssertion) {
				thenResult.DoesNotAllocateUTXOs().
					HasNoChange().
					HasFee(1)
			},
		},
		"user has funded exactly the transaction and fee for bigger size of tx by himself": {
			targetSatoshis: -2,
			txSize:         transactionSizeForHigherFee,
			outputCount:    oneOutput,

			expectations: func(thenResult testabilities.SuccessFundingResultAssertion) {
				thenResult.DoesNotAllocateUTXOs().
					HasNoChange().
					HasFee(2)
			},
		},
		"user has funded by himself more then the transaction and fee": {
			targetSatoshis: -1001,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,

			expectations: func(thenResult testabilities.SuccessFundingResultAssertion) {
				thenResult.DoesNotAllocateUTXOs().
					HasChangeCount(1).ForAmount(1000).
					HasFee(1)
			},
		},
		"user has funded by himself the transaction but not the fee": {
			possessedUTXOs: 1,

			targetSatoshis: 0,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,

			expectations: func(thenResult testabilities.SuccessFundingResultAssertion) {
				thenResult.HasAllocatedUTXOs().ForTotalAmount(1).
					HasNoChange().
					HasFee(1)
			},
		},
		"user has funded by himself the transaction and part of the fee": {
			possessedUTXOs: 1,

			targetSatoshis: -1,
			txSize:         transactionSizeForHigherFee,
			outputCount:    oneOutput,

			expectations: func(thenResult testabilities.SuccessFundingResultAssertion) {
				thenResult.HasAllocatedUTXOs().ForTotalAmount(1).
					HasNoChange().
					HasFee(2)
			},
		},
	}
	for name, test := range testCasesForFundingWithoutAllocatingUTXO {
		t.Run(name, func(t *testing.T) {
			// given:
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			// and:
			funder := given.NewFunderService()

			// and:
			basket := given.BasketFor(testusers.Alice).ThatPrefersSingleChange()

			// and:
			given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(test.possessedUTXOs).P2PKH().Stored()

			// when:
			result, err := funder.Fund(ctx, test.targetSatoshis, test.txSize, test.outputCount, basket, testusers.Alice.ID, nil, nil, false, false)

			// then:
			test.expectations(then.Result(result).WithoutError(err))
		})
	}

	testCasesFundWholeTransaction := map[string]struct {
		havingUTXOsInDB func(testabilities.FunderFixture, *entity.OutputBasket)
		targetSatoshis  satoshi.Value
		txSize          uint64
		outputCount     uint64
		expectations    func(testabilities.SuccessFundingResultAssertion)
	}{
		"target satoshis and fee are equal to the only one utxo satoshis": {
			havingUTXOsInDB: func(given testabilities.FunderFixture, basket *entity.OutputBasket) {
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(101).P2PKH().Stored()
			},

			targetSatoshis: 100,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,

			expectations: func(thenResult testabilities.SuccessFundingResultAssertion) {
				thenResult.HasAllocatedUTXOs().RowIndexes(0).
					HasNoChange().
					HasFee(1)
			},
		},
		"adding utxo can increase the fee": {
			havingUTXOsInDB: func(given testabilities.FunderFixture, basket *entity.OutputBasket) {
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(102).P2PKH().Stored()
			},

			targetSatoshis: 100,
			txSize:         999,
			outputCount:    oneOutput,

			expectations: func(thenResult testabilities.SuccessFundingResultAssertion) {
				thenResult.HasAllocatedUTXOs().RowIndexes(0).
					HasFee(2).
					HasNoChange()
			},
		},
		"user has a lot of small utxo to they will cover the target sats and fee": {
			havingUTXOsInDB: func(given testabilities.FunderFixture, basket *entity.OutputBasket) {
				// Funder is collecting utxos by 1000 rows, so we need to have more than 1000 utxos to test this case.
				for range 1600 {
					given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(1).P2PKH().Stored()
				}
			},

			targetSatoshis: 1363,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,

			expectations: func(thenResult testabilities.SuccessFundingResultAssertion) {
				thenResult.HasAllocatedUTXOs().ForTotalAmount(1600).
					HasNoChange()
			},
		},
		"user has single big utxo and basket is aiming for smallest number of changes": {
			havingUTXOsInDB: func(given testabilities.FunderFixture, basket *entity.OutputBasket) {
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(10101).P2PKH().Stored()
			},

			targetSatoshis: 100,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,

			expectations: func(thenResult testabilities.SuccessFundingResultAssertion) {
				thenResult.HasAllocatedUTXOs().RowIndexes(0).
					HasFee(1).
					HasChangeCount(1).ForAmount(10000)
			},
		},
		"allocate biggest utxos first": {
			havingUTXOsInDB: func(given testabilities.FunderFixture, basket *entity.OutputBasket) {
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(200).P2PKH().Stored()
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(100).P2PKH().Stored()
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(10101).P2PKH().Stored()
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(1).P2PKH().Stored()
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(300).P2PKH().Stored()
			},

			targetSatoshis: 100,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,

			expectations: func(thenResult testabilities.SuccessFundingResultAssertion) {
				thenResult.HasAllocatedUTXOs().RowIndexes(2).
					HasFee(1).
					HasChangeCount(1).ForAmount(10000)
			},
		},
		"allocate several utxos and calculate the change from them": {
			havingUTXOsInDB: func(given testabilities.FunderFixture, basket *entity.OutputBasket) {
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(200).P2PKH().Stored()
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(100).P2PKH().Stored()
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(1).P2PKH().Stored()
				given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(300).P2PKH().Stored()
			},

			targetSatoshis: 549,
			txSize:         smallTransactionSize,
			outputCount:    oneOutput,

			expectations: func(thenResult testabilities.SuccessFundingResultAssertion) {
				thenResult.HasAllocatedUTXOs().RowIndexes(0, 1, 3).
					HasFee(1).
					HasChangeCount(1).ForAmount(50)
			},
		},
	}
	for name, test := range testCasesFundWholeTransaction {
		t.Run(name, func(t *testing.T) {
			// given:
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			// and:
			funder := given.NewFunderService()

			// and:
			basket := given.BasketFor(testusers.Alice).ThatPrefersSingleChange()

			// and:
			test.havingUTXOsInDB(given, basket)

			// when:
			result, err := funder.Fund(ctx, test.targetSatoshis, test.txSize, test.outputCount, basket, testusers.Alice.ID, nil, nil, false, false)

			// then:
			test.expectations(then.Result(result).WithoutError(err))
		})
	}

	t.Run("adding change increases the fee", func(t *testing.T) {
		// given:
		given, then, cleanup := testabilities.New(t)
		defer cleanup()

		// and:
		funder := given.NewFunderService()

		// and:
		basket := given.BasketFor(testusers.Alice).ThatPrefersSingleChange()

		// when:
		result, err := funder.Fund(ctx, -102, 990, noOutputs, basket, testusers.Alice.ID, nil, nil, false, false)

		// then:
		then.Result(result).WithoutError(err).
			HasChangeCount(1).ForAmount(100).
			HasFee(2)
	})

	t.Run("adding change will increase the fee so that there won't be any change, so we're giving extra fee to miner", func(t *testing.T) {
		// given:
		given, then, cleanup := testabilities.New(t)
		defer cleanup()

		// and:
		funder := given.NewFunderService()

		// and:
		basket := given.BasketFor(testusers.Alice).ThatPrefersSingleChange()

		// when:
		result, err := funder.Fund(ctx, -2, 999, oneOutput, basket, testusers.Alice.ID, nil, nil, false, false)

		// then:
		then.Result(result).WithoutError(err).
			HasFee(2).
			HasNoChange()
	})

	t.Run("produce single change when basket NumberOfDesiredUTXOs is 0", func(t *testing.T) {
		// given:
		given, then, cleanup := testabilities.New(t)
		defer cleanup()

		// and:
		funder := given.NewFunderService()

		// and:
		basket := given.BasketFor(testusers.Alice).WithNumberOfDesiredUTXOs(0)

		// when:
		result, err := funder.Fund(ctx, -5001, smallTransactionSize, noOutputs, basket, testusers.Alice.ID, nil, nil, false, false)

		// then:
		then.Result(result).WithoutError(err).
			HasChangeCount(1).ForAmount(5000)
	})

	t.Run("produce single change when basket NumberOfDesiredUTXOs is negative (value: -5)", func(t *testing.T) {
		// given:
		given, then, cleanup := testabilities.New(t)
		defer cleanup()

		// and:
		funder := given.NewFunderService()

		// and:
		basket := given.BasketFor(testusers.Alice).WithNumberOfDesiredUTXOs(-5)

		// when:
		result, err := funder.Fund(ctx, -5001, smallTransactionSize, noOutputs, basket, testusers.Alice.ID, nil, nil, false, false)

		// then:
		then.Result(result).WithoutError(err).
			HasChangeCount(1).ForAmount(5000)
	})

	t.Run("produce single change when user has already utxo number equal to desired basket NumberOfDesiredUTXOs", func(t *testing.T) {
		// given:
		given, then, cleanup := testabilities.New(t)
		defer cleanup()

		// and:
		funder := given.NewFunderService()

		desiredNumber := 10

		// and:
		basket := given.BasketFor(testusers.Alice).WithNumberOfDesiredUTXOs(desiredNumber)

		for range desiredNumber {
			given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(100).P2PKH().Stored()
		}

		// when:
		result, err := funder.Fund(ctx, -5001, smallTransactionSize, noOutputs, basket, testusers.Alice.ID, nil, nil, false, false)

		// then:
		then.Result(result).WithoutError(err).
			HasChangeCount(1).ForAmount(5000)
	})

	t.Run("don't include UTXOs in Sending state that results in insufficient funds", func(t *testing.T) {
		// given:
		given, _, cleanup := testabilities.New(t)
		defer cleanup()

		// and:
		funder := given.NewFunderService()

		// and:
		basket := given.BasketFor(testusers.Alice).ThatPrefersSingleChange()
		const targetSatoshis = 250

		// and:
		given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(100).P2PKH().WithStatus(wdk.UTXOStatusMined).Stored()
		given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(200).P2PKH().WithStatus(wdk.UTXOStatusSending).Stored()

		// when:
		_, err := funder.Fund(ctx, targetSatoshis, smallTransactionSize, oneOutput, basket, testusers.Alice.ID, nil, nil, false, false)

		// then:
		require.Error(t, err)
	})

	t.Run("include UTXOs in Sending state", func(t *testing.T) {
		// given:
		given, then, cleanup := testabilities.New(t)
		defer cleanup()

		// and:
		funder := given.NewFunderService()

		// and:
		basket := given.BasketFor(testusers.Alice).ThatPrefersSingleChange()
		const targetSatoshis = 250

		// and:
		given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(100).P2PKH().WithStatus(wdk.UTXOStatusMined).Stored()
		given.UTXO().InBasket(basket).OwnedBy(testusers.Alice).WithSatoshis(200).P2PKH().WithStatus(wdk.UTXOStatusSending).Stored()

		// when:
		result, err := funder.Fund(ctx, targetSatoshis, smallTransactionSize, oneOutput, basket, testusers.Alice.ID, nil, nil, true, false)

		// then:
		require.NoError(t, err)

		then.Result(result).WithoutError(err).HasAllocatedUTXOs().RowIndexes(0, 1)
	})

	testCasesSplitUserProvidedInputIntoChanges := map[string]struct {
		expectedChangeValue           int
		expectedNumberOfChangeOutputs int
	}{
		"change (value: 249) below minimum desired utxo creates single output": {
			expectedChangeValue:           249,
			expectedNumberOfChangeOutputs: 1,
		},
		"change (value: 250) below minimum desired utxo creates single output": {
			expectedChangeValue:           250,
			expectedNumberOfChangeOutputs: 1,
		},
		"change equal to minimum desired utxo creates single output": {
			expectedChangeValue:           1000,
			expectedNumberOfChangeOutputs: 1,
		},
		"change (value 1001) below 125% of minimum desired utxo creates single output": {
			expectedChangeValue:           1001,
			expectedNumberOfChangeOutputs: 1,
		},
		"change (value 1249) below 125% of minimum desired utxo creates single output": {
			expectedChangeValue:           1249,
			expectedNumberOfChangeOutputs: 1,
		},
		"change equal to 125% of minimum desired utxo creates two outputs": {
			expectedChangeValue:           1250,
			expectedNumberOfChangeOutputs: 2,
		},
		"change equal to 200% of minimum desired utxo creates two outputs": {
			expectedChangeValue:           2000,
			expectedNumberOfChangeOutputs: 2,
		},
		"change above 200% but below 225% of minimum desired utxo creates two outputs": {
			expectedChangeValue:           2249,
			expectedNumberOfChangeOutputs: 2,
		},
		"change above 225% of minimum desired utxo creates three outputs": {
			expectedChangeValue:           2250,
			expectedNumberOfChangeOutputs: 3,
		},
		"change equal to (minimum desired utxo) times (number of desired utxo) creates desired utxo number of changes": {
			expectedChangeValue:           3000,
			expectedNumberOfChangeOutputs: 3,
		},
		"change above the (minimum desired utxo) times (number of desired utxo) creates desired utxo number of changes": {
			expectedChangeValue:           10000,
			expectedNumberOfChangeOutputs: 3,
		},
	}
	for name, test := range testCasesSplitUserProvidedInputIntoChanges {
		t.Run(name, func(t *testing.T) {
			// given:
			fee := 1

			// and: targetSatoshis should cover the fee and the expected change value
			// and it must be negative to simulate that user provides by himself the inputs to cover those values.
			targetSatoshis := -satoshi.MustAdd(test.expectedChangeValue, fee)

			// and:
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			// and:
			funder := given.NewFunderService()

			// and: basket with limit of 3 outputs
			basket := given.BasketFor(testusers.Alice).WithNumberOfDesiredUTXOs(3)

			// when:
			result, err := funder.Fund(ctx, targetSatoshis, smallTransactionSize, noOutputs, basket, testusers.Alice.ID, nil, nil, false, false)

			// then:
			then.Result(result).WithoutError(err).
				HasChangeCount(test.expectedNumberOfChangeOutputs).ForAmount(test.expectedChangeValue)
		})
	}
}

func TestFunderSQLFundChangeManagement(t *testing.T) {
	const smallTransactionSize = 44
	const noOutputs = uint64(0)
	const oneOutput = uint64(1)
	ctx := t.Context()

	t.Run("cap: output count never exceeds MaxChangeOutputsPerTransaction even when numberOfDesiredUTXOs is much larger", func(t *testing.T) {
		// given:
		given, then, cleanup := testabilities.New(t)
		defer cleanup()

		// and: basket wants 100 UTXOs, each at 1000 sats
		funderSvc := given.NewFunderService()
		basket := given.BasketFor(testusers.Alice).WithNumberOfDesiredUTXOs(100)

		// and: a single large UTXO covers the full desired pool value many times over
		// 100_000_000 sats = 1 BSV
		// when:
		result, err := funderSvc.Fund(ctx, -100_000_001, smallTransactionSize, noOutputs, basket, testusers.Alice.ID, nil, nil, false, false)

		// then: change outputs must be capped at MaxChangeOutputsPerTx, not 100
		then.Result(result).WithoutError(err).
			HasChangeCount(int(defs.DefaultChangeBasket().MaxChangeOutputsPerTx)). //nolint:gosec // uint64 to int conversion is safe, value is bounded by config
			ForAmount(100_000_000)
	})

	t.Run("cap: gradual pool build-up – each transaction adds at most MaxChangeOutputsPerTransaction net new outputs", func(t *testing.T) {
		// given: fee rate 1 sat/kb (default)
		const desiredUTXOs = 20
		const largeInputSats = satoshi.Value(-10_000_001)

		given, then, cleanup := testabilities.New(t)
		defer cleanup()
		funderSvc := given.NewFunderService()

		// and: basket with 20 desired UTXOs – 0 already in the pool, so numberOfDesiredUTXOs-existing = 20
		basket := given.BasketFor(testusers.Alice).WithNumberOfDesiredUTXOs(desiredUTXOs)

		// when:
		result, err := funderSvc.Fund(ctx, largeInputSats, smallTransactionSize, noOutputs, basket, testusers.Alice.ID, nil, nil, false, false)

		// then: only MaxChangeOutputsPerTransaction outputs created, not 20
		then.Result(result).WithoutError(err)

		require.LessOrEqual(t, result.ChangeOutputsCount, defs.DefaultChangeBasket().MaxChangeOutputsPerTx,
			"ChangeOutputsCount should not exceed MaxChangeOutputsPerTx")
	})

	t.Run("cap: SetMaxChangeOutputsPerTx takes effect on next Fund call", func(t *testing.T) {
		// given: funder with default cap (8), basket wants 20 UTXOs
		const desiredUTXOs = 20
		const newCap = uint64(3)

		given, then, cleanup := testabilities.New(t)
		defer cleanup()
		funderSvc := given.NewFunderService()
		basket := given.BasketFor(testusers.Alice).WithNumberOfDesiredUTXOs(desiredUTXOs)

		// when: cap is lowered to 3 at runtime
		funderSvc.SetMaxChangeOutputsPerTx(newCap)

		result, err := funderSvc.Fund(ctx, -10_000_001, smallTransactionSize, noOutputs, basket, testusers.Alice.ID, nil, nil, false, false)

		// then: new cap of 3 is respected
		then.Result(result).WithoutError(err)
		require.LessOrEqual(t, result.ChangeOutputsCount, newCap,
			"ChangeOutputsCount should not exceed the runtime-updated cap")
	})

	t.Run("dust floor: change below dust floor (at high fee rate) is given to the miner instead of creating an output", func(t *testing.T) {
		// given:
		const feeRate = int64(500)

		given, then, cleanup := testabilities.New(t)
		defer cleanup()

		funderSvc := given.NewFunderServiceWithFeeRate(feeRate)
		basket := given.BasketFor(testusers.Alice).ThatPrefersSingleChange()

		// when:
		result, err := funderSvc.Fund(ctx, -50, smallTransactionSize, oneOutput, basket, testusers.Alice.ID, nil, nil, false, false)

		// then: no change output created; extra sats go to the miner (ChangeAmount > 0 is fine)
		then.Result(result).WithoutError(err)
		require.Zero(t, result.ChangeOutputsCount, "expected 0 change outputs when change is below dustFloor")
		require.Positive(t, int(result.ChangeAmount), "sub-dust change amount should be positive (given to miner as fee)")
		require.Less(t, int(result.ChangeAmount), 192, "change amount must be below dustFloor (192 at 500 sat/kb)")
	})

	t.Run("dust floor: large change at high fee rate still produces outputs above the dust floor", func(t *testing.T) {
		// given:
		const feeRate = int64(110)

		given, then, cleanup := testabilities.New(t)
		defer cleanup()

		funderSvc := given.NewFunderServiceWithFeeRate(feeRate)
		// basket with 3 desired UTXOs, minimum 1000 sats each
		basket := given.BasketFor(testusers.Alice).WithNumberOfDesiredUTXOs(3)

		// when: over-fund by 3500 sats – enough for 3 change outputs × 1000+ sats each
		result, err := funderSvc.Fund(ctx, -3501, smallTransactionSize, noOutputs, basket, testusers.Alice.ID, nil, nil, false, false)

		then.Result(result).WithoutError(err)
		// dustFloor = 44 sats; each output must be well above that
		require.Positive(t, int(result.ChangeAmount), "change must be positive")
		if result.ChangeOutputsCount > 0 {
			avgPerOutput := int64(result.ChangeAmount) / int64(result.ChangeOutputsCount) //nolint:gosec // test values are small
			require.GreaterOrEqual(t, avgPerOutput, int64(44),
				"average change per output must be >= dustFloor (44 sats at 110 sat/kb)")
		}
	})

	t.Run("dust floor: reduces output count so no individual output falls below floor", func(t *testing.T) {
		const feeRate = int64(1000)

		given, then, cleanup := testabilities.New(t)
		defer cleanup()

		funderSvc := given.NewFunderServiceWithFeeRate(feeRate)

		basket := &entity.OutputBasket{
			UserID:                  testusers.Alice.ID,
			Name:                    "default",
			NumberOfDesiredUTXOs:    5,
			MinimumDesiredUTXOValue: 400,
		}

		result, err := funderSvc.Fund(ctx, -801, smallTransactionSize, noOutputs, basket, testusers.Alice.ID, nil, nil, false, false)

		then.Result(result).WithoutError(err)

		// Verify that no individual output would be below the dust floor
		if result.ChangeOutputsCount > 0 {
			perOutput := int64(result.ChangeAmount) / int64(result.ChangeOutputsCount) //nolint:gosec // test values are small
			// dustFloor at 1000 sat/kb = ceil(192/1000*1000)*2 = 192*2 = 384
			require.GreaterOrEqualf(t, perOutput, int64(384),
				"per-output value %d must be >= dustFloor 384 at 1000 sat/kb", perOutput)
		}
	})
}

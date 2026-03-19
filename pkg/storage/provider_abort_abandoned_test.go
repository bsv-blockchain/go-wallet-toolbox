package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestAbortAbandoned_AbortByStatus(t *testing.T) {
	tests := map[string]struct {
		setup func(given testabilities.TxGeneratorFixture) (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction)
	}{
		"unprocessed tx": {
			setup: func(given testabilities.TxGeneratorFixture) (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction) {
				return given.Unprocessed()
			},
		},
		"unsigned tx": {
			setup: func(given testabilities.TxGeneratorFixture) (createActionResult *wdk.StorageCreateActionResult, signedTx *transaction.Transaction) {
				return given.Created()
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()
			activeStorage := given.Provider().
				WithRandomizer(randomizer.NewTestRandomizer()).
				WithFailAbandonedMinTxAge(0). // this way we can abort immediately
				GORM()

			// and:
			const (
				satoshisToInternalize = 5000
				satoshisToSend        = 1000
			)

			givenTxGenerator := given.Action(activeStorage).
				WithSatoshisToInternalize(satoshisToInternalize).
				WithSatoshisToSend(satoshisToSend)

			createActionResult, _ := test.setup(givenTxGenerator)

			// when:
			err := activeStorage.AbortAbandoned(t.Context())

			// then:
			require.NoError(t, err)

			// and db state:
			thenDBState := testabilities.ThenDBState(t, activeStorage)
			thenDBState.HasUserTransactionByReference(testusers.Alice, createActionResult.Reference).
				WithStatus(wdk.TxStatusFailed)

			// and:
			testabilities.ThenFunds(t, testusers.Alice, activeStorage).
				ShouldBeAbleToReserveSatoshis(satoshisToInternalize)
		})
	}
}

func TestAbortAbandoned_HandleMultipleTransaction(t *testing.T) {
	const (
		sendingTxCount        = 10 // they should remain unaffected
		toBeAbortedCount      = 10 // they should be aborted
		satoshisToInternalize = 5000
		satoshisToSend        = 1000
	)

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		WithFailAbandonedMinTxAge(0). // this way we can abort immediately
		GORM()

	sendingTxReferences := make([]string, 0, sendingTxCount)
	for i := range sendingTxCount {
		createActionResult, _ := given.Action(activeStorage).
			WithSatoshisToInternalize(uint64(satoshisToInternalize + i)).
			WithSatoshisToSend(uint64(satoshisToSend + i)).
			WillFailOnBroadcast(). // NOTE: this way the tx will remain in 'sending' status
			Processed()

		sendingTxReferences = append(sendingTxReferences, createActionResult.Reference)
	}

	internalizedSatsPerAbandonedTx := uint64(0)
	for i := range toBeAbortedCount {
		toInternalize := uint64(2*satoshisToInternalize + i)
		internalizedSatsPerAbandonedTx += toInternalize

		_, _ = given.Action(activeStorage).
			WithSatoshisToInternalize(toInternalize).
			WithSatoshisToSend(uint64(2*satoshisToSend + i)).
			Unprocessed()
	}

	// when:
	err := activeStorage.AbortAbandoned(t.Context())

	// then:
	require.NoError(t, err)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	for _, reference := range sendingTxReferences {
		thenDBState.HasUserTransactionByReference(testusers.Alice, reference).
			WithStatus(wdk.TxStatusSending) // The sending tx should remain unaffected
	}

	// and:
	testabilities.ThenFunds(t, testusers.Alice, activeStorage).
		ShouldBeAbleToReserveSatoshis(internalizedSatsPerAbandonedTx - 1) // -1 to additional fee
}

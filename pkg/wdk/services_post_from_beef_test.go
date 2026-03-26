package wdk_test

import (
	"fmt"
	"testing"

	"github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestPostBeefResult(t *testing.T) {
	tests := map[string]struct {
		given   wdk.PostFromBeefResult
		success bool
	}{
		"single success": {
			given: wdk.PostFromBeefResult{
				&wdk.PostFromBEEFServiceResult{
					Name:             "service1",
					PostedBEEFResult: &wdk.PostedBEEF{},
				},
			},
			success: true,
		},
		"single error": {
			given: wdk.PostFromBeefResult{
				&wdk.PostFromBEEFServiceResult{
					Name:  "service1",
					Error: fmt.Errorf("some-error"),
				},
			},
			success: false,
		},
		"single no postedBEEF": {
			given: wdk.PostFromBeefResult{
				&wdk.PostFromBEEFServiceResult{
					Name: "service1",
				},
			},
			success: false,
		},
		"success and error": {
			given: wdk.PostFromBeefResult{
				&wdk.PostFromBEEFServiceResult{
					Name:             "service1",
					PostedBEEFResult: &wdk.PostedBEEF{},
				},
				&wdk.PostFromBEEFServiceResult{
					Name:  "service2",
					Error: fmt.Errorf("some-error"),
				},
			},
			success: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			success := test.given.Success()

			// then:
			require.Equal(t, test.success, success)
		})
	}
}

func TestAggregated(t *testing.T) {
	t.Run("success, one service, one txid", func(t *testing.T) {
		// given:
		txID := mockTxID(1)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name: "service1",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultSuccess,
							TxID:   txID,
						},
					},
				},
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID})

		// then:
		require.Len(t, aggregated, 1)

		aggTxID, ok := aggregated[txID]
		require.True(t, ok)

		require.Equal(t, 1, aggTxID.SuccessCount)
		require.Equal(t, 0, aggTxID.DoubleSpendCount)
		require.Equal(t, 0, aggTxID.StatusErrorCount)
		require.Equal(t, 0, aggTxID.ServiceErrorCount)
		require.Empty(t, aggTxID.CompetingTxs)
		require.Len(t, aggTxID.TxIDResults, 1)
		require.Equal(t, wdk.AggregatedPostedTxIDSuccess, aggTxID.Status)
	})

	t.Run("already known, one service, one txid", func(t *testing.T) {
		// given:
		txID := mockTxID(1)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name: "service1",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultAlreadyKnown,
							TxID:   txID,
						},
					},
				},
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID})

		// then:
		require.Len(t, aggregated, 1)

		aggTxID, ok := aggregated[txID]
		require.True(t, ok)

		require.Equal(t, 1, aggTxID.SuccessCount)
		require.Equal(t, 0, aggTxID.DoubleSpendCount)
		require.Equal(t, 0, aggTxID.StatusErrorCount)
		require.Equal(t, 0, aggTxID.ServiceErrorCount)
		require.Empty(t, aggTxID.CompetingTxs)
		require.Len(t, aggTxID.TxIDResults, 1)
		require.Equal(t, wdk.AggregatedPostedTxIDSuccess, aggTxID.Status)
	})

	t.Run("success, two services, one txid", func(t *testing.T) {
		// given:
		txID := mockTxID(1)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name: "service1",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultSuccess,
							TxID:   txID,
						},
					},
				},
			},
			&wdk.PostFromBEEFServiceResult{
				Name: "service2",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultSuccess,
							TxID:   txID,
						},
					},
				},
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID})

		// then:
		require.Len(t, aggregated, 1)

		aggTxID, ok := aggregated[txID]
		require.True(t, ok)

		require.Equal(t, 2, aggTxID.SuccessCount)
		require.Equal(t, 0, aggTxID.DoubleSpendCount)
		require.Equal(t, 0, aggTxID.StatusErrorCount)
		require.Equal(t, 0, aggTxID.ServiceErrorCount)
		require.Empty(t, aggTxID.CompetingTxs)
		require.Len(t, aggTxID.TxIDResults, 2)
		require.Equal(t, wdk.AggregatedPostedTxIDSuccess, aggTxID.Status)
	})

	t.Run("success, two services, two txIDs - one per service", func(t *testing.T) {
		// given:
		txID1 := mockTxID(1)
		txID2 := mockTxID(2)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name: "service1",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultSuccess,
							TxID:   txID1,
						},
					},
				},
			},
			&wdk.PostFromBEEFServiceResult{
				Name: "service2",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultSuccess,
							TxID:   txID2,
						},
					},
				},
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID1, txID2})

		// then:
		require.Len(t, aggregated, 2)

		for _, txid := range []string{txID1, txID2} {
			require.Contains(t, aggregated, txid)
			require.Equal(t, 1, aggregated[txid].SuccessCount)
			require.Equal(t, 0, aggregated[txid].DoubleSpendCount)
			require.Equal(t, 0, aggregated[txid].StatusErrorCount)
			require.Equal(t, 0, aggregated[txid].ServiceErrorCount)
			require.Empty(t, aggregated[txid].CompetingTxs)
			require.Len(t, aggregated[txid].TxIDResults, 1)
			require.Equal(t, wdk.AggregatedPostedTxIDSuccess, aggregated[txid].Status)
		}
	})

	t.Run("success, two services, two txIDs - two per service", func(t *testing.T) {
		// given:
		txID1 := mockTxID(1)
		txID2 := mockTxID(2)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name: "service1",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultSuccess,
							TxID:   txID1,
						},
						{
							Result: wdk.PostedTxIDResultSuccess,
							TxID:   txID2,
						},
					},
				},
			},
			&wdk.PostFromBEEFServiceResult{
				Name: "service2",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultSuccess,
							TxID:   txID1,
						},
						{
							Result: wdk.PostedTxIDResultSuccess,
							TxID:   txID2,
						},
					},
				},
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID1, txID2})

		// then:
		require.Len(t, aggregated, 2)

		for _, txid := range []string{txID1, txID2} {
			require.Contains(t, aggregated, txid)
			require.Equal(t, 2, aggregated[txid].SuccessCount)
			require.Equal(t, 0, aggregated[txid].DoubleSpendCount)
			require.Equal(t, 0, aggregated[txid].StatusErrorCount)
			require.Equal(t, 0, aggregated[txid].ServiceErrorCount)
			require.Empty(t, aggregated[txid].CompetingTxs)
			require.Len(t, aggregated[txid].TxIDResults, 2)
			require.Equal(t, wdk.AggregatedPostedTxIDSuccess, aggregated[txid].Status)
		}
	})

	t.Run("status error, one service, one txid", func(t *testing.T) {
		// given:
		txID := mockTxID(1)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name: "service1",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultError,
							TxID:   txID,
						},
					},
				},
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID})

		// then:
		require.Len(t, aggregated, 1)

		aggTxID, ok := aggregated[txID]
		require.True(t, ok)

		require.Equal(t, 0, aggTxID.SuccessCount)
		require.Equal(t, 0, aggTxID.DoubleSpendCount)
		require.Equal(t, 1, aggTxID.StatusErrorCount)
		require.Equal(t, 0, aggTxID.ServiceErrorCount)
		require.Empty(t, aggTxID.CompetingTxs)
		require.Len(t, aggTxID.TxIDResults, 1)
		require.Equal(t, wdk.AggregatedPostedTxIDInvalidTx, aggTxID.Status)
	})

	t.Run("error, one service with error, one txid", func(t *testing.T) {
		// given:
		txID := mockTxID(1)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name:  "service1",
				Error: fmt.Errorf("some-error"),
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID})

		// then:
		require.Len(t, aggregated, 1)

		aggTxID, ok := aggregated[txID]
		require.True(t, ok)

		require.Equal(t, 0, aggTxID.SuccessCount)

		require.Equal(t, 0, aggTxID.DoubleSpendCount)
		require.Equal(t, 0, aggTxID.StatusErrorCount)
		require.Equal(t, 0, aggTxID.ServiceErrorCount)
		require.Empty(t, aggTxID.CompetingTxs)
		require.Empty(t, aggTxID.TxIDResults)
		require.Equal(t, wdk.AggregatedPostedTxIDServiceError, aggTxID.Status)
	})

	t.Run("error, one service with error, one txid", func(t *testing.T) {
		// given:
		txID := mockTxID(1)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name: "service1",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultError,
							TxID:   txID,
							Error:  fmt.Errorf("some-error"),
						},
					},
				},
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID})

		// then:
		require.Len(t, aggregated, 1)

		aggTxID, ok := aggregated[txID]
		require.True(t, ok)

		require.Equal(t, 0, aggTxID.SuccessCount)

		require.Equal(t, 0, aggTxID.DoubleSpendCount)
		require.Equal(t, 0, aggTxID.StatusErrorCount)
		require.Equal(t, 1, aggTxID.ServiceErrorCount)
		require.Empty(t, aggTxID.CompetingTxs)
		require.Len(t, aggTxID.TxIDResults, 1)
		require.Equal(t, wdk.AggregatedPostedTxIDServiceError, aggTxID.Status)
	})

	t.Run("success, one service success, one service error, one txid", func(t *testing.T) {
		// given:
		txID := mockTxID(1)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name: "service1",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultError,
							TxID:   txID,
							Error:  fmt.Errorf("some-error"),
						},
					},
				},
			},
			&wdk.PostFromBEEFServiceResult{
				Name: "service2",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultSuccess,
							TxID:   txID,
						},
					},
				},
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID})

		// then:
		require.Len(t, aggregated, 1)

		aggTxID, ok := aggregated[txID]
		require.True(t, ok)

		require.Equal(t, 1, aggTxID.SuccessCount)
		require.Equal(t, 0, aggTxID.DoubleSpendCount)
		require.Equal(t, 0, aggTxID.StatusErrorCount)
		require.Equal(t, 1, aggTxID.ServiceErrorCount)
		require.Empty(t, aggTxID.CompetingTxs)
		require.Len(t, aggTxID.TxIDResults, 2)
		require.Equal(t, wdk.AggregatedPostedTxIDSuccess, aggTxID.Status)
	})

	t.Run("double spend, one service, one txid", func(t *testing.T) {
		// given:
		txID := mockTxID(1)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name: "service1",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result:      wdk.PostedTxIDResultError,
							TxID:        txID,
							DoubleSpend: true,
						},
					},
				},
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID})

		// then:
		require.Len(t, aggregated, 1)

		aggTxID, ok := aggregated[txID]
		require.True(t, ok)

		require.Equal(t, 0, aggTxID.SuccessCount)
		require.Equal(t, 1, aggTxID.DoubleSpendCount)
		require.Equal(t, 0, aggTxID.StatusErrorCount)
		require.Equal(t, 0, aggTxID.ServiceErrorCount)
		require.Empty(t, aggTxID.CompetingTxs)
		require.Len(t, aggTxID.TxIDResults, 1)
		require.Equal(t, wdk.AggregatedPostedTxIDDoubleSpend, aggTxID.Status)
	})

	t.Run("double spend, two services, one txid with competingTx", func(t *testing.T) {
		// given:
		txID := mockTxID(1)
		competingTxID := mockTxID(2)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name: "service1",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result:       wdk.PostedTxIDResultError,
							TxID:         txID,
							DoubleSpend:  true,
							CompetingTxs: []string{competingTxID},
						},
					},
				},
			},
			&wdk.PostFromBEEFServiceResult{
				Name: "service2",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result:       wdk.PostedTxIDResultError,
							TxID:         txID,
							DoubleSpend:  true,
							CompetingTxs: []string{competingTxID},
						},
					},
				},
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID})

		// then:
		require.Len(t, aggregated, 1)

		aggTxID, ok := aggregated[txID]
		require.True(t, ok)

		require.Equal(t, 0, aggTxID.SuccessCount)
		require.Equal(t, 2, aggTxID.DoubleSpendCount)
		require.Equal(t, 0, aggTxID.StatusErrorCount)
		require.Equal(t, 0, aggTxID.ServiceErrorCount)
		require.Len(t, aggTxID.CompetingTxs, 1)
		require.Len(t, aggTxID.TxIDResults, 2)
		require.Equal(t, wdk.AggregatedPostedTxIDDoubleSpend, aggTxID.Status)
	})

	t.Run("double spend, two services, one success, one double spend", func(t *testing.T) {
		// given:
		txID := mockTxID(1)
		result := wdk.PostFromBeefResult{
			&wdk.PostFromBEEFServiceResult{
				Name: "service1",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result: wdk.PostedTxIDResultSuccess,
							TxID:   txID,
						},
					},
				},
			},
			&wdk.PostFromBEEFServiceResult{
				Name: "service2",
				PostedBEEFResult: &wdk.PostedBEEF{
					TxIDResults: []wdk.PostedTxID{
						{
							Result:      wdk.PostedTxIDResultError,
							TxID:        txID,
							DoubleSpend: true,
						},
					},
				},
			},
		}

		// when:
		aggregated := result.Aggregated([]string{txID})

		// then:
		require.Len(t, aggregated, 1)

		aggTxID, ok := aggregated[txID]
		require.True(t, ok)

		require.Equal(t, 1, aggTxID.SuccessCount)
		require.Equal(t, 1, aggTxID.DoubleSpendCount)
		require.Equal(t, 0, aggTxID.StatusErrorCount)
		require.Equal(t, 0, aggTxID.ServiceErrorCount)
		require.Empty(t, aggTxID.CompetingTxs)
		require.Len(t, aggTxID.TxIDResults, 2)
		require.Equal(t, wdk.AggregatedPostedTxIDDoubleSpend, aggTxID.Status)
	})

	t.Run("empty result", func(t *testing.T) {
		// given:
		txID := mockTxID(1)
		result := wdk.PostFromBeefResult{}

		// when:
		aggregated := result.Aggregated([]string{txID})

		// then:
		require.Len(t, aggregated, 1)

		aggTxID, ok := aggregated[txID]
		require.True(t, ok)

		require.Equal(t, 0, aggTxID.SuccessCount)
		require.Equal(t, 0, aggTxID.DoubleSpendCount)
		require.Equal(t, 0, aggTxID.StatusErrorCount)
		require.Equal(t, 0, aggTxID.ServiceErrorCount)
		require.Empty(t, aggTxID.CompetingTxs)
		require.Empty(t, aggTxID.TxIDResults)
		require.Equal(t, wdk.AggregatedPostedTxIDServiceError, aggTxID.Status)
	})
}

func mockTxID(num uint64) string {
	return testabilities.GivenTX().WithInput(2 + num).WithP2PKHOutput(1 + num).ID().String()
}

package wdk

// AggregatedPostedTxIDStatus represents the aggregated status of postBEEF process for single txid
type AggregatedPostedTxIDStatus string

// Possible values for AggregatedPostedTxIDStatus
const (
	AggregatedPostedTxIDSuccess      AggregatedPostedTxIDStatus = "success"
	AggregatedPostedTxIDDoubleSpend  AggregatedPostedTxIDStatus = "doubleSpend"
	AggregatedPostedTxIDInvalidTx    AggregatedPostedTxIDStatus = "invalidTx"
	AggregatedPostedTxIDServiceError AggregatedPostedTxIDStatus = "serviceError"
)

// AggregatedPostedTxID represents postBEEF result, aggregated from all broadcasters for particular TxID
type AggregatedPostedTxID struct {
	TxID              string
	TxIDResults       []*PostedTxID
	Status            AggregatedPostedTxIDStatus
	SuccessCount      int
	DoubleSpendCount  int
	StatusErrorCount  int
	ServiceErrorCount int
	CompetingTxs      map[string]struct{}
}

// AggregatedPostBEEF is a map of AggregatedPostedTxID results, indexed by txid
type AggregatedPostBEEF map[string]*AggregatedPostedTxID

func (a AggregatedPostBEEF) getOrDefault(txid string) *AggregatedPostedTxID {
	if agg, ok := a[txid]; ok {
		return agg
	}

	agg := &AggregatedPostedTxID{
		TxID:         txid,
		CompetingTxs: make(map[string]struct{}),
	}
	a[txid] = agg

	return agg
}

func (a AggregatedPostBEEF) summarize(txID string) {
	agg, ok := a[txID]
	if !ok {
		agg = &AggregatedPostedTxID{
			TxID:         txID,
			CompetingTxs: make(map[string]struct{}),
			Status:       AggregatedPostedTxIDServiceError,
		}
		a[txID] = agg
		return
	}

	switch {
	case agg.DoubleSpendCount > 0:
		agg.Status = AggregatedPostedTxIDDoubleSpend
	case agg.SuccessCount > 0:
		agg.Status = AggregatedPostedTxIDSuccess
	case agg.ServiceErrorCount > 0:
		agg.Status = AggregatedPostedTxIDServiceError
	default:
		agg.Status = AggregatedPostedTxIDInvalidTx
	}
}

func newAggregatedPostBEEF(results PostBeefResult, txids []string) AggregatedPostBEEF {
	aggregatedTxs := make(AggregatedPostBEEF)

	for _, result := range results {
		if !result.Success() {
			continue
		}

		mapped := make(map[string]*PostedTxID)
		for _, txIDResult := range result.PostedBEEFResult.TxIDResults {
			mapped[txIDResult.TxID] = &txIDResult
		}

		for _, txid := range txids {
			txIDResult, ok := mapped[txid]
			if !ok {
				continue
			}

			agg := aggregatedTxs.getOrDefault(txid)

			agg.TxIDResults = append(agg.TxIDResults, txIDResult)

			switch {
			case txIDResult.Result == PostedTxIDResultSuccess, txIDResult.Result == PostedTxIDResultAlreadyKnown:
				agg.SuccessCount++
			case txIDResult.DoubleSpend:
				agg.DoubleSpendCount++
				for _, competingTx := range txIDResult.CompetingTxs {
					agg.CompetingTxs[competingTx] = struct{}{}
				}
			case txIDResult.Error != nil:
				agg.ServiceErrorCount++
			default:
				agg.StatusErrorCount++
			}
		}
	}

	for _, txid := range txids {
		aggregatedTxs.summarize(txid)
	}

	return aggregatedTxs
}

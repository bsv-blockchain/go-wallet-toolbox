package history

const (
	InternalizeActionHistoryNote = "internalizeAction"
	ProcessActionHistoryNote     = "processAction"
	AggregateResultsHistoryNote  = "aggregateResults"
)

func UserIDHistoryAttr(userID int) map[string]any {
	return map[string]any{
		"userId": userID,
	}
}

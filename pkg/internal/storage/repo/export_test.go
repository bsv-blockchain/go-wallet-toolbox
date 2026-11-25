package repo

// Test-only handles on the unexported implementations, so external tests and benchmarks can run
// both implementations of the same operation against the same database.
var (
	UpsertKnownTxSingleStatement = upsertKnownTxSingleStatement
	UpsertKnownTxReadModifyWrite = upsertKnownTxReadModifyWrite
)

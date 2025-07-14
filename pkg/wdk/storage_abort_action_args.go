package wdk

// AbortActionArgs defines the arguments for aborting a wallet action.
type AbortActionArgs struct {
	// Reference is the unique identifier for the action to be aborted.
	Reference *string
}

// AbortActionResult defines the result of an abort action operation.
type AbortActionResult struct {
	// Aborted indicates whether the action was successfully aborted.
	Aborted bool
}

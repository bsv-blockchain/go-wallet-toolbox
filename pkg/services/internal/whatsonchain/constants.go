package whatsonchain

import "time"

const (
	// Retries is the number of retries the client should make when trying to query for the chain's resource
	Retries = 2
	// RetriesWaitTime is the duration to wait to retry a failed call to outside service
	RetriesWaitTime = 2 * time.Second
)

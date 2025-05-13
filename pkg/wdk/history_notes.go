package wdk

import "time"

// ReqHistoryNote is the history representation of the request
type ReqHistoryNote struct {
	When *time.Time `json:"when"`
	What string     `json:"what"`
	Args map[string]any
}

// Notes represents a list of ReqHistoryNote
type Notes []ReqHistoryNote

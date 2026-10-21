package wdk

import "time"

// AdminStatsCounts holds a counter over the last day, week (7 days), month (30 days) and in total.
type AdminStatsCounts struct {
	Day   int64 `json:"day"`
	Week  int64 `json:"week"`
	Month int64 `json:"month"`
	Total int64 `json:"total"`
}

// AdminStats is a storage-wide usage summary meant for storage administrators.
type AdminStats struct {
	RequestedBy  string           `json:"requestedBy"`
	When         time.Time        `json:"when"`
	Users        AdminStatsCounts `json:"users"`
	Transactions AdminStatsCounts `json:"transactions"`
	// TxByStatus counts transactions per status; "failed" excludes transactions without txid (see TxAbandoned).
	TxByStatus map[TxStatus]AdminStatsCounts `json:"txByStatus"`
	// TxAbandoned counts failed transactions that never got a txid.
	TxAbandoned AdminStatsCounts `json:"txAbandoned"`
	// SatoshisDefault sums spendable change outputs of completed transactions.
	SatoshisDefault AdminStatsCounts `json:"satoshisDefault"`
	// SatoshisOther sums spendable non-change outputs in a basket of completed transactions.
	SatoshisOther AdminStatsCounts `json:"satoshisOther"`
	Baskets       AdminStatsCounts `json:"baskets"`
	Labels        AdminStatsCounts `json:"labels"`
	Tags          AdminStatsCounts `json:"tags"`
	// ServicesStats is nil when the configured services do not record call history.
	ServicesStats ServicesCallHistory `json:"servicesStats,omitempty"`
}

// ServicesCallHistory is the call history of every service method, keyed by method name (e.g. "MerklePath").
type ServicesCallHistory map[string]ServiceCallHistory

// ServiceCallHistory is the call history of a single service method across all its providers.
type ServiceCallHistory struct {
	ServiceName       string                         `json:"serviceName"`
	HistoryByProvider map[string]ProviderCallHistory `json:"historyByProvider"`
}

// ProviderCallHistory is the call history of a single service method of a single provider.
type ProviderCallHistory struct {
	ServiceName  string `json:"serviceName"`
	ProviderName string `json:"providerName"`
	// Calls are the most recent calls, newest first.
	Calls []ServiceCall `json:"calls"`
	// TotalCounts are counts since the services were created.
	TotalCounts ServiceCallHistoryCounts `json:"totalCounts"`
	// ResetCounts[0] is the current interval; each reset prepends a new one.
	ResetCounts []ServiceCallHistoryCounts `json:"resetCounts"`
}

// ServiceCall describes a single call to a service provider.
type ServiceCall struct {
	When    time.Time `json:"when"`
	Msecs   int64     `json:"msecs"`
	Success bool      `json:"success"`
	// Result is a short summary of a non-error failure, e.g. "not_found".
	Result string `json:"result,omitempty"`
	// Error is the error message when the call failed with an error.
	Error string `json:"error,omitempty"`
}

// ServiceCallHistoryCounts counts service calls over the interval Since..Until.
type ServiceCallHistoryCounts struct {
	Success int       `json:"success"`
	Failure int       `json:"failure"`
	Error   int       `json:"error"`
	Since   time.Time `json:"since"`
	Until   time.Time `json:"until"`
}

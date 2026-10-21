package servicequeue

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/go-softwarelab/common/pkg/is"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	maxCallHistory = 32
	maxResetCounts = 32

	outcomeOK    = "ok"
	outcomeError = "error"
)

// outcomeOf classifies a service call result: "ok", the expected misses "not_found", "empty"
// and "canceled" (logged at debug), or "error" for real failures (logged at warn).
func outcomeOf(err error) (outcome string, level slog.Level) {
	switch {
	case err == nil:
		return outcomeOK, slog.LevelDebug
	case errors.Is(err, wdk.ErrNotFoundError):
		return "not_found", slog.LevelDebug
	case errors.Is(err, ErrEmptyResult):
		return "empty", slog.LevelDebug
	case errors.Is(err, context.Canceled):
		// the caller went away; not the service's fault
		return "canceled", slog.LevelDebug
	default:
		return outcomeError, slog.LevelWarn
	}
}

func recordResult[R any](h *CallHistory, method string, start time.Time, result *NamedResult[R]) {
	if h == nil || result == nil {
		return
	}
	var err error
	if result.IsError() {
		err = result.GetError()
	} else if is.Nil(result.MustGetValue()) {
		err = ErrEmptyResult
	}
	h.Record(method, result.Name(), start, err)
}

// CallHistory records recent calls and outcome counts per service method and provider.
// A nil *CallHistory records nothing.
type CallHistory struct {
	mu       sync.Mutex
	since    time.Time
	byMethod map[string]map[string]*wdk.ProviderCallHistory
}

// NewCallHistory creates an empty CallHistory starting now.
func NewCallHistory() *CallHistory {
	return &CallHistory{
		since:    time.Now(),
		byMethod: make(map[string]map[string]*wdk.ProviderCallHistory),
	}
}

// Record adds one call of the method to the provider, started at start and finished now with err.
func (h *CallHistory) Record(method, provider string, start time.Time, err error) {
	if h == nil {
		return
	}
	outcome, _ := outcomeOf(err)
	now := time.Now()
	call := wdk.ServiceCall{When: start, Msecs: now.Sub(start).Milliseconds()}

	h.mu.Lock()
	defer h.mu.Unlock()

	providers, ok := h.byMethod[method]
	if !ok {
		providers = make(map[string]*wdk.ProviderCallHistory)
		h.byMethod[method] = providers
	}
	ph, ok := providers[provider]
	if !ok {
		ph = &wdk.ProviderCallHistory{
			ServiceName:  method,
			ProviderName: provider,
			TotalCounts:  wdk.ServiceCallHistoryCounts{Since: h.since},
			ResetCounts:  []wdk.ServiceCallHistoryCounts{{Since: h.since}},
		}
		providers[provider] = ph
	}

	total, current := &ph.TotalCounts, &ph.ResetCounts[0]
	switch outcome {
	case outcomeOK:
		call.Success = true
		total.Success++
		current.Success++
	case outcomeError:
		call.Error = err.Error()
		total.Failure++
		total.Error++
		current.Failure++
		current.Error++
	default:
		call.Result = outcome
		total.Failure++
		current.Failure++
	}
	total.Until, current.Until = now, now

	ph.Calls = slices.Insert(ph.Calls, 0, call)
	if len(ph.Calls) > maxCallHistory {
		ph.Calls = ph.Calls[:maxCallHistory]
	}
}

// Snapshot returns a copy of the history. With reset, every provider starts a new ResetCounts interval.
func (h *CallHistory) Snapshot(reset bool) wdk.ServicesCallHistory {
	if h == nil {
		return nil
	}
	now := time.Now()

	h.mu.Lock()
	defer h.mu.Unlock()

	result := make(wdk.ServicesCallHistory, len(h.byMethod))
	for method, providers := range h.byMethod {
		sh := wdk.ServiceCallHistory{ServiceName: method, HistoryByProvider: make(map[string]wdk.ProviderCallHistory, len(providers))}
		for name, ph := range providers {
			cp := *ph
			cp.Calls = slices.Clone(ph.Calls)
			cp.ResetCounts = slices.Clone(ph.ResetCounts)
			sh.HistoryByProvider[name] = cp

			if reset {
				ph.ResetCounts[0].Until = now
				ph.ResetCounts = slices.Insert(ph.ResetCounts, 0, wdk.ServiceCallHistoryCounts{Since: now, Until: now})
				if len(ph.ResetCounts) > maxResetCounts {
					ph.ResetCounts = ph.ResetCounts[:maxResetCounts]
				}
			}
		}
		result[method] = sh
	}
	return result
}

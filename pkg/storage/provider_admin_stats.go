package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// AdminStats returns storage-wide usage counters (last day, 7 days, 30 days and total) and,
// when the services record it (e.g. services.WalletServices), the service call history.
// Like the TS toolbox, every call starts a new ResetCounts interval in the service call history.
// It does not check permissions: callers exposing it must restrict it to administrators.
func (p *Provider) AdminStats(ctx context.Context, requestedBy string) (*wdk.AdminStats, error) {
	now := time.Now()
	stats, err := p.repo.AdminStats(ctx, now.Add(-24*time.Hour), now.Add(-7*24*time.Hour), now.Add(-30*24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to collect admin stats: %w", err)
	}
	stats.RequestedBy = requestedBy
	stats.When = now

	if h, ok := p.services.(interface {
		ServicesCallHistory(reset bool) wdk.ServicesCallHistory
	}); ok {
		stats.ServicesStats = h.ServicesCallHistory(true)
	}
	return stats, nil
}

package actions

import (
	"context"

	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
)

// GroupCommitter runs units of database work from concurrent callers inside shared transactions
// (see package groupcommit). work runs on the batch's GroupScope; solo is the caller's ungrouped
// code path, used when the work cannot be grouped. A nil GroupCommitter means group commit is off.
type GroupCommitter interface {
	Do(ctx context.Context, work func(ctx context.Context, scope GroupScope) error, solo func(ctx context.Context) error) error
}

// GroupScope is what the units of one group commit batch run on: the shared transaction and
// repositories bound to it. The repositories are built at most once per batch, on first use
// (building them is not cheap), and shared by the batch's units, which run one at a time. Like
// unit-of-work handles they never retry: the shared transaction is owned by the committer. Every
// repository call takes the unit's own context.
type GroupScope struct {
	Tx    *gorm.DB
	repos *txRepos
}

// NewGroupScope builds the scope of a batch over its shared transaction.
func NewGroupScope(tx *gorm.DB) GroupScope {
	return GroupScope{Tx: tx, repos: &txRepos{}}
}

// Repos returns the repositories bound to the shared transaction.
func (s GroupScope) Repos() Providers {
	if s.repos.r == nil {
		s.repos.r = repo.NewSQLRepositories(s.Tx, dbretry.NoRetry())
	}
	return s.repos
}

type txRepos struct {
	r *repo.Repositories
}

func (p *txRepos) TransactionsRepo() TransactionsRepo { return p.r.Transactions }
func (p *txRepos) OutputRepo() OutputRepo             { return p.r.Outputs }
func (p *txRepos) KnownTxRepo() KnownTxRepo           { return p.r.KnownTx }
func (p *txRepos) UTXORepo() UTXORepo                 { return p.r.UTXOs }
func (p *txRepos) BasketRepo() BasketRepo             { return p.r.OutputBaskets }
func (p *txRepos) CommissionRepo() CommissionRepo     { return p.r.Commission }
func (p *txRepos) KeyValueRepo() KeyValueRepo         { return p.r.KeyValue }

// grouped runs work through the group committer when group commit is on, and solo - the
// unchanged ungrouped path - otherwise.
func grouped(ctx context.Context, group GroupCommitter, work func(ctx context.Context, repos Providers) error, solo func(ctx context.Context) error) error {
	if group == nil {
		return solo(ctx)
	}
	return group.Do(ctx, func(ctx context.Context, scope GroupScope) error {
		return work(ctx, scope.Repos())
	}, solo)
}

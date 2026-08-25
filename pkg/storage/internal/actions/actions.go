package actions

import (
	"context"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/service"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Actions struct {
	*create
	*internalize
	*process
	*synchronizeTxStatuses
	*listOutputs
	*listActions
	*abortAction
	*getBeef
}

// ThroughputConfig is the resolved runtime configuration of the throughput
// UTXO-management strategy (defs.StrategyThroughput). The zero value means the
// strategy is disabled and funding behaves exactly as before.
type ThroughputConfig struct {
	Enabled            bool
	Denomination       uint64
	SpendPolicy        defs.SpendPolicy
	PoolBasket         string
	ReserveBasket      string
	FanoutOutputsPerTx uint64
	// TargetTPS is the createAction rate this deployment is provisioned for.
	// It sizes the delayed-broadcast pool: acceptance must keep pace with
	// creation, because outputs only become spendable once accepted.
	TargetTPS uint64
}

// broadcasterSizing derives the delayed-broadcast pool from the configured
// rate. A post takes on the order of a few hundred milliseconds, so sustaining
// N tx/s needs roughly N/3 concurrent posts; the buffer holds a few seconds of
// creation so a burst does not spill to the (much slower) cron fallback.
// Non-throughput deployments get the package defaults.
func (t ThroughputConfig) broadcasterSizing() service.Sizing {
	if !t.Enabled || t.TargetTPS == 0 {
		return service.Sizing{}
	}
	workers := int(min(t.TargetTPS/2, 256))
	if workers < service.BackgroundBroadcasterWorkerCount {
		workers = service.BackgroundBroadcasterWorkerCount
	}
	return service.Sizing{
		Workers:     workers,
		ChannelSize: workers * 400,
	}
}

// resolveBroadcasterSizing combines the operator's explicit sizing with the one
// derived from the throughput strategy. Resolution is per field and an explicit
// value wins, so a deployment that is not on the throughput strategy - where the
// derivation yields nothing - can still widen just the queue for bursty traffic.
// Fields left at zero fall through to the package defaults in service.Sizing.
func resolveBroadcasterSizing(explicit defs.BackgroundBroadcaster, throughput ThroughputConfig) service.Sizing {
	sizing := throughput.broadcasterSizing()
	if explicit.Workers > 0 {
		sizing.Workers = int(explicit.Workers)
	}
	if explicit.ChannelSize > 0 {
		sizing.ChannelSize = int(explicit.ChannelSize)
	}
	return sizing
}

// BroadcasterQueueStats reports the delayed-broadcast queue occupancy, so the
// provider can expose it as a gauge. Depth is what predicts an overflow; the
// overflow counter only reports one after it already happened.
func (a *Actions) BroadcasterQueueStats() (depth, capacity int) {
	return a.backgroundBroadcaster.QueueStats()
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	fnd *funder.SQL,
	commission defs.Commission,
	repos *repo.Repositories,
	uow UnitOfWork,
	randomizer wdk.Randomizer,
	services wdk.Services,
	syncTxStatusesConfig defs.SynchronizeTxStatuses,
	beefVerifier wdk.BeefVerifier,
	scriptsVerifier wdk.ScriptsVerifier,
	txBroadcastedChannel chan<- wdk.CurrentTxStatus,
	throughput ThroughputConfig,
	broadcasterCfg defs.BackgroundBroadcaster,
) *Actions {
	abortAction := newAbortAction(logger, repos.Transactions, repos.Outputs, repos.UTXOs, repos.KnownTx, uow)

	processAction := newProcessAction(
		ctx,
		logger,
		repos.Transactions,
		commission,
		repos.Outputs,
		repos.KnownTx,
		repos.Commission,
		repos.UTXOs,
		uow,
		services,
		randomizer,
		beefVerifier,
		scriptsVerifier,
		txBroadcastedChannel,
		resolveBroadcasterSizing(broadcasterCfg, throughput),
		syncTxStatusesConfig.MaxRebroadcastAttempts,
		abortAction,
	)

	return &Actions{
		create: newCreateAction(
			logger,
			fnd,
			repos.DB,
			commission,
			repos.OutputBaskets,
			repos.Transactions,
			repos.Transactions,
			repos.UTXOs,
			repos.Outputs,
			repos.KnownTx,
			repos.Commission,
			randomizer,
			services,
			beefVerifier,
			scriptsVerifier,
			throughput,
			abortAction,
		),
		internalize: newInternalizeAction(
			logger,
			repos.Transactions,
			repos.OutputBaskets,
			repos.KnownTx,
			repos.Outputs,
			uow,
			randomizer,
			beefVerifier,
			scriptsVerifier,
			services,
			processAction,
		),
		process:               processAction,
		listOutputs:           newListOutputs(logger, repos.Outputs, repos.KnownTx, repos.Transactions),
		synchronizeTxStatuses: newSynchronizeTxStatuses(logger, syncTxStatusesConfig, services, repos.KnownTx, repos.KeyValue, repos.Transactions, repos.Outputs, uow),
		listActions:           newListActions(logger, repos.Transactions, repos.Outputs, repos.KnownTx, repos.OutputBaskets),
		abortAction:           abortAction,
		getBeef:               newGetBeef(logger, repos.KnownTx, services),
	}
}

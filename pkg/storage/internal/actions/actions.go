package actions

import (
	"context"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
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

func New(
	ctx context.Context,
	logger *slog.Logger,
	funder funder.Funder,
	commission defs.Commission,
	repos *repo.Repositories,
	randomizer wdk.Randomizer,
	services wdk.Services,
	syncTxStatusesConfig defs.SynchronizeTxStatuses,
	beefVerifier wdk.BeefVerifier,
	txBroadcastedChannel chan<- wdk.CurrentTxStatus,
) *Actions {
	return &Actions{
		create: newCreateAction(
			logger,
			funder,
			commission,
			repos.OutputBaskets,
			repos.Transactions,
			repos.Outputs,
			repos.KnownTx,
			repos.Commission,
			randomizer,
			services,
			beefVerifier,
		),
		internalize: newInternalizeAction(
			logger,
			repos.Transactions,
			repos.OutputBaskets,
			repos.KnownTx,
			repos.Outputs,
			randomizer,
			beefVerifier,
			services,
		),
		process: newProcessAction(
			ctx,
			logger,
			repos.Transactions,
			commission,
			repos.Outputs,
			repos.KnownTx,
			repos.Commission,
			repos.UTXOs,
			services,
			randomizer,
			beefVerifier,
			txBroadcastedChannel,
		),
		listOutputs:           newListOutputs(logger, repos.Outputs, repos.KnownTx, repos.Transactions),
		synchronizeTxStatuses: newSynchronizeTxStatuses(logger, syncTxStatusesConfig, services, repos.KnownTx, repos.KeyValue, repos.Transactions),
		listActions:           newListActions(logger, repos.Transactions, repos.Outputs, repos.KnownTx, repos.OutputBaskets),
		abortAction:           newAbortAction(logger, repos.Transactions, repos.Outputs, repos.UTXOs, repos.KnownTx),
		getBeef:               newGetBeef(logger, repos.KnownTx, services),
	}
}

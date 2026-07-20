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
) *Actions {
	processAction := newProcessAction(
		ctx,
		logger,
		repos.Transactions,
		commission,
		repos.Outputs,
		repos.ProvenTxReqRepo,
		repos.Commission,
		repos.UTXOs,
		uow,
		services,
		randomizer,
		beefVerifier,
		scriptsVerifier,
		txBroadcastedChannel,
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
			repos.ProvenTxReqRepo,
			repos.Commission,
			randomizer,
			services,
			beefVerifier,
			scriptsVerifier,
		),
		internalize: newInternalizeAction(
			logger,
			repos.Transactions,
			repos.OutputBaskets,
			repos.ProvenTxReqRepo,
			repos.Outputs,
			uow,
			randomizer,
			beefVerifier,
			scriptsVerifier,
			services,
			processAction.backgroundBroadcaster,
		),
		process:               processAction,
		listOutputs:           newListOutputs(logger, repos.Outputs, repos.ProvenTxReqRepo, repos.Transactions),
		synchronizeTxStatuses: newSynchronizeTxStatuses(logger, syncTxStatusesConfig, services, repos.ProvenTxReqRepo, repos.KeyValue, repos.Transactions, repos.Outputs, uow),
		listActions:           newListActions(logger, repos.Transactions, repos.Outputs, repos.ProvenTxReqRepo, repos.OutputBaskets),
		abortAction:           newAbortAction(logger, repos.Transactions, repos.Outputs, repos.UTXOs, repos.ProvenTxReqRepo, uow),
		getBeef:               newGetBeef(logger, repos.ProvenTxReqRepo, services),
	}
}

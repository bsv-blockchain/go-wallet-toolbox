package actions

import (
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type Actions struct {
	*create
	*internalize
	*process
	*synchronizeTxStatuses
	*listOutputs
}

func New(logger *slog.Logger, funder funder.Funder, commission defs.Commission, repos *repo.Repositories, randomizer wdk.Randomizer, services wdk.Services, syncTxStatusesConfig defs.SynchronizeTxStatuses) *Actions {
	return &Actions{
		create: newCreateAction(
			logger,
			funder,
			commission,
			repos.OutputBaskets,
			repos.Transactions,
			repos.Outputs,
			repos.ProvenTxReq,
			randomizer,
		),
		internalize: newInternalizeAction(
			logger,
			repos.Transactions,
			repos.OutputBaskets,
			repos.ProvenTxReq,
			randomizer,
		),
		process:               newProcessAction(logger, repos.Transactions, repos.Outputs, repos.ProvenTxReq, services),
		synchronizeTxStatuses: newSynchronizeTxStatuses(logger, syncTxStatusesConfig, services, repos.ProvenTxReq),
		listOutputs:           newListOutputs(logger, repos.Outputs, repos.ProvenTxReq),
	}
}

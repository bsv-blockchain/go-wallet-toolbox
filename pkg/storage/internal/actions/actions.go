package actions

import (
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/repo"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type Actions struct {
	*create
	*internalize
	*process
}

func New(logger *slog.Logger, funder Funder, commission defs.Commission, repos *repo.Repositories, randomizer wdk.Randomizer) *Actions {
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
		process: newProcessAction(logger, repos.Transactions, repos.Outputs, repos.ProvenTxReq),
	}
}

package tasks

import (
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
)

type CheckForProofsTask struct {
	logger  *slog.Logger
	counter uint64
}

func NewCheckForProofsTask(logger *slog.Logger) TaskInterface {
	return &CheckForProofsTask{
		logger:  logging.Child(logger, "check_for_proofs"),
		counter: 0,
	}
}

func (t *CheckForProofsTask) Run() {
	// TODO: implement
}

package actions

import (
	"context"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"log/slog"
	"sync"
)

type synchronizeTxStatuses struct {
	lock   sync.Mutex
	logger *slog.Logger
}

func newSynchronizeTxStatuses(logger *slog.Logger) *synchronizeTxStatuses {
	return &synchronizeTxStatuses{
		logger: logging.Child(logger, "synchronize_tx_statuses"),
	}
}

func (s *synchronizeTxStatuses) SynchronizeTxStatuses(_ context.Context) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.logger.Error("not implemented yet")
	return nil
}

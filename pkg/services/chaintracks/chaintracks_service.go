package chaintracks

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/gormstorage"
)

// Service provides core functionality for the Chaintracks service with logging and configuration support.
type Service struct {
	logger *slog.Logger
	config defs.ChaintracksServiceConfig

	storage Storage

	cancelCtx          context.CancelFunc
	makeAvailableOnce  sync.Once
	shiftLiveHeadersWG sync.WaitGroup

	available   bool
	availableMu sync.RWMutex
}

// NewService creates and returns a new Service instance initialized with the provided logger and configuration.
// Returns an error if the given config is invalid according to its validation rules.
func NewService(logger *slog.Logger, config defs.ChaintracksServiceConfig) (*Service, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid chaintracks service config: %w", err)
	}

	// NOTE: This creates an in-memory SQLite DB which is not persistent.
	// NOTE: This is acceptable for this case, when it's no big deal to re-sync data on restart.
	// TODO: Add config options to allow persistent storage backends.
	dbConfig := gormstorage.InMemorySQLiteDBConfig()
	storage, err := gormstorage.NewProvider(logger, gormstorage.WithDBConfig(dbConfig))
	if err != nil {
		return nil, fmt.Errorf("failed to create chaintracks storage provider: %w", err)
	}

	return &Service{
		logger:  logging.Child(logger, "chaintracks_service"),
		config:  config,
		storage: storage,
	}, nil
}

// GetChain returns the configured BSV network for the service.
func (s *Service) GetChain() defs.BSVNetwork {
	return s.config.Chain
}

// MakeAvailable initializes and marks the service as available, starting background workers as needed.
func (s *Service) MakeAvailable(parentCtx context.Context) (err error) {
	s.makeAvailableOnce.Do(func() {
		s.logger.Info("Chaintracks service - making available")

		ctx, cancel := context.WithCancel(parentCtx)
		s.cancelCtx = cancel

		if err := s.storage.Migrate(ctx); err != nil {
			err = fmt.Errorf("failed to migrate chaintracks storage: %w", err)
			s.logger.Error("Chaintracks service", slog.String("error", err.Error()))
			return
		}

		// TODO: Implement make available logic here

		err = s.shiftLiveHeaders(ctx)
		if err != nil {
			err = fmt.Errorf("error during initial live headers shift: %w", err)
			s.logger.Error("Chaintracks service", slog.String("error", err.Error()))
			return
		}

		s.shiftLiveHeadersWG.Add(1)
		go s.shiftLiveHeadersWorker(ctx)
		s.setAvailable(true)

		s.logger.Info("Chaintracks service - now available")
	})

	return
}

// Available returns true if the service is currently marked as available, false otherwise.
func (s *Service) Available() bool {
	s.availableMu.RLock()
	defer s.availableMu.RUnlock()
	return s.available
}

func (s *Service) setAvailable(value bool) {
	s.availableMu.Lock()
	defer s.availableMu.Unlock()
	s.available = value
}

// Destroy gracefully shuts down the service, cancels background tasks, and waits for all workers to complete.
func (s *Service) Destroy() {
	s.logger.Info("Chaintracks service - destroying")

	if s.cancelCtx != nil {
		s.cancelCtx()
	}
	s.shiftLiveHeadersWG.Wait()

	s.setAvailable(false)

	s.logger.Info("Chaintracks service - destroyed")
}

func (s *Service) shiftLiveHeadersWorker(ctx context.Context) {
	defer s.shiftLiveHeadersWG.Done()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Chaintracks service - shift live headers worker stopping due to context cancellation")
			return
		default:
			err := s.shiftLiveHeaders(ctx)
			if err != nil {
				s.logger.Error("Chaintracks service - error shifting live headers", slog.String("error", err.Error()))
			}
		}
	}
}

func (s *Service) shiftLiveHeaders(_ context.Context) error {
	// TODO: Implement live headers shifting logic here
	time.Sleep(100 * time.Millisecond) // this mimics work being done - will be removed when real work is implemented

	return nil
}

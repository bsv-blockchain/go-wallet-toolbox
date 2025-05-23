package testabilities

import (
	"log/slog"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/monitor"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type MonitorFixture interface {
	Daemon() *monitor.Daemon
}

func Given(t testing.TB) MonitorFixture {
	return &monitorFixture{
		t:           t,
		logger:      logging.NewTestLogger(t),
		require:     require.New(t),
		mockStorage: &MockStorage{},
	}
}

type monitorFixture struct {
	t           testing.TB
	logger      *slog.Logger
	require     *require.Assertions
	mockStorage *MockStorage
	daemon      *monitor.Daemon
}

func (m *monitorFixture) Daemon() *monitor.Daemon {
	connectionString := "file:monitor.test.sqlite?mode=memory"

	db, err := gorm.Open(sqlite.Open(connectionString), &gorm.Config{})
	m.require.NoError(err, "failed to connect to sqlite in-memory database")

	daemon, err := monitor.NewDaemonWithGORMLocker(m.t.Context(), m.logger, m.mockStorage, db)
	m.require.NoError(err)

	m.daemon = daemon

	return daemon
}

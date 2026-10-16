package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testmode"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestMakeAvailable(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORMWithCleanDatabase()

	// when:
	tableSettings, err := activeStorage.MakeAvailable(t.Context())

	// then:
	require.NoError(t, err)

	assert.Equal(t, fixtures.StorageName, tableSettings.StorageName)
	assert.Equal(t, fixtures.StorageIdentityKey, tableSettings.StorageIdentityKey)
	assert.Equal(t, defs.NetworkTestnet, tableSettings.Chain)
	assert.Equal(t, 1024, tableSettings.MaxOutputScript)

	// The engine is reported with the cross-implementation spelling, so TypeScript storage
	// clients accept the settings (conformance/vectors/wallet/storage uses "SQLite").
	expectedDBType := wdk.StorageDBTypeSQLite
	if _, isPostgres := testmode.GetMode().(*testmode.PostgresMode); isPostgres {
		expectedDBType = wdk.StorageDBTypePostgres
	}
	assert.Equal(t, expectedDBType, tableSettings.DbType)
}

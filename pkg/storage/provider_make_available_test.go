package storage_test

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
}

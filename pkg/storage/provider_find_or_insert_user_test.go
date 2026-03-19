package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
)

func TestFindOrInsertUser(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	userIdentityKey := "03f17660f611ce531402a2ce1e070380b6fde57aca211d707bfab27bce42d86beb"

	// and:
	activeStorage := given.Provider().GORMWithCleanDatabase()

	// when:
	tableUser, err := activeStorage.FindOrInsertUser(t.Context(), userIdentityKey)

	// then:
	require.NoError(t, err)

	assert.True(t, tableUser.IsNew)
	assert.Equal(t, userIdentityKey, tableUser.User.IdentityKey)

	// and when:
	tableUser, err = activeStorage.FindOrInsertUser(t.Context(), userIdentityKey)

	// then:
	require.NoError(t, err)

	assert.False(t, tableUser.IsNew)
	assert.Equal(t, userIdentityKey, tableUser.User.IdentityKey)
}

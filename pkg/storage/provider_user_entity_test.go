package storage_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
)

type seededUsers struct {
	alice entity.User
	bob   entity.User
}

func TestUserFindByID(t *testing.T) {
	// given:
	provider, users := seedDbWithUsers(t)

	// when:
	usersFound, err := provider.UserEntity().Read().ID(users.alice.ID).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, usersFound, 1)
	assert.Equal(t, users.alice.ID, usersFound[0].ID)
}

func TestUserFindByIdentityKey(t *testing.T) {
	// given:
	provider, users := seedDbWithUsers(t)

	// when:
	usersFound, err := provider.UserEntity().Read().IdentityKey().Equals(users.alice.IdentityKey).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, usersFound, 1)
	assert.Equal(t, users.alice.IdentityKey, usersFound[0].IdentityKey)
}

func TestUserFindByActiveStorage(t *testing.T) {
	// given:
	provider, users := seedDbWithUsers(t)

	// when:
	found, err := provider.UserEntity().
		Read().
		ActiveStorage().Equals(users.alice.ActiveStorage).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, users.alice.ID, found[0].ID)
	assert.Equal(t, users.alice.ActiveStorage, found[0].ActiveStorage)
}

func TestUserCountAll(t *testing.T) {
	// given:
	provider, users := seedDbWithUsers(t)

	// when:
	count, err := provider.UserEntity().Read().IdentityKey().In(users.alice.IdentityKey, users.bob.IdentityKey).Count(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestUserCountByIdentityKey(t *testing.T) {
	// given:
	provider, users := seedDbWithUsers(t)

	// when:
	count, err := provider.UserEntity().Read().IdentityKey().Equals(users.bob.IdentityKey).Count(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestUserUpdateActiveStorage(t *testing.T) {
	// given:
	provider, users := seedDbWithUsers(t)
	newStorage := "updated-storage"
	now := time.Now()

	// when:
	err := provider.UserEntity().Update(t.Context(), &entity.UserUpdateSpecification{
		ID:            users.alice.ID,
		ActiveStorage: &newStorage,
	})

	// then:
	require.NoError(t, err)
	usersFound, err := provider.UserEntity().Read().ID(users.alice.ID).Find(t.Context())
	require.NoError(t, err)
	require.Len(t, usersFound, 1)
	assert.Equal(t, "updated-storage", usersFound[0].ActiveStorage)
	assert.WithinDuration(t, now, usersFound[0].UpdatedAt, time.Second)
}

func TestUserUpdateAutoUpdatedAt(t *testing.T) {
	// given:
	provider, users := seedDbWithUsers(t)
	repo := provider.UserEntity()

	oldUser := users.alice
	oldUpdatedAt := oldUser.UpdatedAt

	// when:
	newStorage := "new-storage"
	err := repo.Update(t.Context(), &entity.UserUpdateSpecification{
		ID:            oldUser.ID,
		ActiveStorage: &newStorage,
	})

	// then:
	require.NoError(t, err)

	updatedUser, err := repo.Read().ID(oldUser.ID).Find(t.Context())
	require.NoError(t, err)
	require.Len(t, updatedUser, 1)

	assert.Equal(t, "new-storage", updatedUser[0].ActiveStorage)

	assert.True(t, updatedUser[0].UpdatedAt.After(oldUpdatedAt),
		"expected UpdatedAt to be newer than before")
	assert.WithinDuration(t, time.Now(), updatedUser[0].UpdatedAt, time.Second,
		"expected UpdatedAt to be set close to current time")
}

func TestUserFindPaged(t *testing.T) {
	// given:
	provider, _ := seedDbWithUsers(t)

	// when:
	usersFound, err := provider.UserEntity().Read().Paged(4, 4, false).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, usersFound, 4)
	assert.Equal(t, 1, usersFound[0].ID)
	assert.Equal(t, 2, usersFound[1].ID)
	assert.Equal(t, 3, usersFound[2].ID)
	require.Equal(t, 4, usersFound[3].ID)
}

func seedDbWithUsers(t testing.TB) (*storage.Provider, seededUsers) {
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	alice := entity.User{
		IdentityKey:   "alice-" + uuid.New().String(),
		ActiveStorage: "storage-1",
	}
	require.NoError(t, provider.UserEntity().Create(t.Context(), &alice))
	found, err := provider.UserEntity().Read().IdentityKey().Equals(alice.IdentityKey).Find(t.Context())
	require.NoError(t, err)
	require.Len(t, found, 1)
	alice.ID = found[0].ID

	bob := entity.User{
		IdentityKey:   "bob-" + uuid.New().String(),
		ActiveStorage: "storage-2",
	}
	require.NoError(t, provider.UserEntity().Create(t.Context(), &bob))
	found, err = provider.UserEntity().Read().IdentityKey().Equals(bob.IdentityKey).Find(t.Context())
	require.NoError(t, err)
	require.Len(t, found, 1)
	bob.ID = found[0].ID

	return provider, seededUsers{alice: alice, bob: bob}
}

package managed

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Storage struct {
	wdk.WalletStorageProvider
	Settings *wdk.TableSettings
	User     *wdk.TableUser
}

func NewManagedStorage(storage wdk.WalletStorageProvider) *Storage {
	return &Storage{
		WalletStorageProvider: storage,
	}
}

func (s *Storage) IsAvailable() bool {
	return s.Settings != nil && s.User != nil
}

func (s *Storage) MakeAvailableStorage(ctx context.Context, identityKey string) (*wdk.TableSettings, error) {
	if s.IsAvailable() {
		return s.Settings, nil
	}
	settings, err := s.MakeAvailable(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to make available storage: %w", err)
	}

	userResponse, err := s.FindOrInsertUser(ctx, identityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to find or insert User to storage %s: %w", settings.StorageName, err)
	}
	if userResponse.User.IdentityKey != identityKey {
		return nil, fmt.Errorf("storage %s returned User with different identity key (%s)", settings.StorageName, userResponse.User.IdentityKey)
	}

	s.Settings = settings
	s.User = &userResponse.User

	return settings, nil
}

func (s *Storage) ThinksItIsActive() bool {
	return s.IsAvailable() && s.User.ActiveStorage == s.Settings.StorageIdentityKey
}

func (s *Storage) ThinksActiveStorageIs(storageIdentityKey string) bool {
	return s.IsAvailable() && s.User.ActiveStorage == storageIdentityKey
}

func (s *Storage) HasStorageIdentityKey(identityKey string) bool {
	return s.IsAvailable() && s.Settings.StorageIdentityKey == identityKey
}

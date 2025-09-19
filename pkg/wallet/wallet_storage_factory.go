package wallet

import (
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func noopCleanup() { /* Do nothing */ }

// StorageProviderFactoryWithWalletReturningCleanupAndError is a function type for creating a WalletStorageProvider.
// It accepts a wallet interface, returning the provider, a cleanup function, and an error if applicable.
type StorageProviderFactoryWithWalletReturningCleanupAndError = func(wallet sdk.Interface) (storage wdk.WalletStorageProvider, cleanup func(), err error)

// StorageProviderFactoryWithWalletReturningCleanup is a function type for creating a WalletStorageProvider.
// It accepts a wallet interface, returning the provider, and a cleanup function.
type StorageProviderFactoryWithWalletReturningCleanup = func(wallet sdk.Interface) (storage wdk.WalletStorageProvider, cleanup func())

// StorageProviderFactoryWithWalletReturningError is a function type for creating a WalletStorageProvider.
// It accepts a wallet interface, returning the provider, and an error if applicable.
type StorageProviderFactoryWithWalletReturningError = func(wallet sdk.Interface) (storage wdk.WalletStorageProvider, err error)

// StorageProviderFactoryWithWallet is a function type for creating a WalletStorageProvider.
// It accepts a wallet interface, returning the provider.
type StorageProviderFactoryWithWallet = func(wallet sdk.Interface) (storage wdk.WalletStorageProvider)

// StorageProviderFactoryWithoutWalletReturningCleanupAndError is a function type for creating a WalletStorageProvider.
// It accepts no arguments, returning the provider, a cleanup function, and an error if applicable.
type StorageProviderFactoryWithoutWalletReturningCleanupAndError = func() (storage wdk.WalletStorageProvider, cleanup func(), err error)

// StorageProviderFactoryWithoutWalletReturningCleanup is a function type for creating a WalletStorageProvider.
// It accepts no arguments, returning the provider, and a cleanup function.
type StorageProviderFactoryWithoutWalletReturningCleanup = func() (storage wdk.WalletStorageProvider, cleanup func())

// StorageProviderFactoryWithoutWalletReturningError is a function type for creating a WalletStorageProvider.
// It accepts no arguments, returning the provider, and an error if applicable.
type StorageProviderFactoryWithoutWalletReturningError = func() (storage wdk.WalletStorageProvider, err error)

// StorageProviderFactoryWithoutWallet is a function type for creating a WalletStorageProvider.
// It accepts no arguments, returning the provider.
type StorageProviderFactoryWithoutWallet = func() (storage wdk.WalletStorageProvider)

// StorageProviderFactory defines an interface for types that create WalletStorageProvider instances in various configurations.
// It is used as a storage factory by NewWithStorageFactory constructor to create a Wallet instance.
type StorageProviderFactory interface {
	StorageProviderFactoryWithWalletReturningCleanupAndError |
		StorageProviderFactoryWithWalletReturningCleanup |
		StorageProviderFactoryWithWalletReturningError |
		StorageProviderFactoryWithWallet |
		StorageProviderFactoryWithoutWalletReturningCleanupAndError |
		StorageProviderFactoryWithoutWalletReturningCleanup |
		StorageProviderFactoryWithoutWalletReturningError |
		StorageProviderFactoryWithoutWallet
}

func toStorageProvider[F StorageProviderFactory](wallet sdk.Interface, factory F) (wdk.WalletStorageProvider, func(), error) {
	if factory == nil {
		return nil, nil, fmt.Errorf("active storage factory must be provided")
	}

	switch f := any(factory).(type) {
	case StorageProviderFactoryWithWalletReturningCleanupAndError:
		storage, cleanup, err := f(wallet)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create storage provider with %T: %w", f, err)
		}
		if cleanup == nil {
			cleanup = noopCleanup
		}
		return storage, cleanup, nil
	case StorageProviderFactoryWithWalletReturningCleanup:
		storage, cleanup := f(wallet)
		if cleanup == nil {
			cleanup = noopCleanup
		}
		return storage, cleanup, nil
	case StorageProviderFactoryWithWalletReturningError:
		storage, err := f(wallet)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create storage provider with %T: %w", f, err)
		}
		return storage, noopCleanup, nil
	case StorageProviderFactoryWithWallet:
		return f(wallet), noopCleanup, nil
	case StorageProviderFactoryWithoutWalletReturningCleanupAndError:
		storage, cleanup, err := f()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create storage provider with %T: %w", f, err)
		}
		if cleanup == nil {
			cleanup = noopCleanup
		}
		return storage, cleanup, nil
	case StorageProviderFactoryWithoutWalletReturningCleanup:
		storage, cleanup := f()
		if cleanup == nil {
			cleanup = noopCleanup
		}
		return storage, cleanup, nil
	case StorageProviderFactoryWithoutWalletReturningError:
		storage, err := f()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create storage provider with %T: %w", f, err)
		}
		return storage, noopCleanup, nil
	case StorageProviderFactoryWithoutWallet:
		return f(), noopCleanup, nil
	default:
		panic(fmt.Errorf("unexpected type (%T) passed without compiler error", factory))
	}
}

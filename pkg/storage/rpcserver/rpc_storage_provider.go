package rpcserver

// DEPRECATED: This file belongs to the legacy JSON-RPC storage remoting layer.
// Use v1adapter + storage.Server instead. See rpc_server.go for full deprecation notice.

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-bsv-middleware/pkg/middleware"
	"github.com/go-softwarelab/common/pkg/is"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

var _ wdk.WalletStorageProvider = (*RPCStorageProvider)(nil)

// RPCStorageProvider wraps a WalletStorageProvider with identity verification.
type RPCStorageProvider struct {
	localProvider wdk.WalletStorageProvider
	log           *slog.Logger
}

// NewRPCStorageProvider creates an RPCStorageProvider that delegates to localProvider.
func NewRPCStorageProvider(logger *slog.Logger, localProvider wdk.WalletStorageProvider) *RPCStorageProvider {
	return &RPCStorageProvider{
		localProvider: localProvider,
		log:           logging.Child(logger, "RPCStorageProvider"),
	}
}

// FindOrInsertUser verifies the caller's identity then delegates to the local provider.
func (p *RPCStorageProvider) FindOrInsertUser(ctx context.Context, identityKey string) (*wdk.FindOrInsertUserResponse, error) {
	err := p.verifyIdentityKey(ctx, identityKey)
	if err != nil {
		return nil, err
	}

	return p.localProvider.FindOrInsertUser(ctx, identityKey) //nolint:wrapcheck // direct delegation to underlying provider
}

func (p *RPCStorageProvider) ensureUserID(ctx context.Context, auth *wdk.AuthID) error {
	user, err := p.localProvider.FindOrInsertUser(ctx, auth.IdentityKey)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	userID := user.User.UserID

	if auth.UserID != nil && *auth.UserID != userID {
		p.log.WarnContext(ctx, "User ID from AuthID argument does not match the one for the identity key",
			logging.UserID(userID),
			slog.Int("providedUserID", *auth.UserID),
			slog.String("identityKey", auth.IdentityKey))
	}

	auth.UserID = &userID
	return nil
}

// Destroy is a no-op on the server side. Remote clients call destroy when
// cleaning up their local wallet instance, but that should not shut down
// the server's storage. Matches the TypeScript StorageServer behavior.
func (p *RPCStorageProvider) Destroy() {}

func (p *RPCStorageProvider) verifyAuthenticated(ctx context.Context) error {
	if middleware.IsNotAuthenticated(ctx) {
		return fmt.Errorf("function may only access authenticated user")
	}
	return nil
}

func (p *RPCStorageProvider) verifyAuthID(ctx context.Context, auth wdk.AuthID) error {
	return p.verifyIdentityKey(ctx, auth.IdentityKey)
}

// verifyAndBindSyncIdentityKey binds the identity whose data a sync request touches
// to the authenticated BRC-103 peer identity.
//
// RequestSyncChunkArgs carries no AuthID; its IdentityKey alone selects the user
// whose wallet data is exported (GetSyncChunk) or written (ProcessSyncChunk).
// Requiring only that the caller be authenticated therefore lets any authenticated
// peer name someone else's identity key and read or corrupt their storage. Callers
// only ever sync their own data, so the authenticated identity is authoritative:
// a blank identityKey is filled in from it and a differing one is rejected.
func (p *RPCStorageProvider) verifyAndBindSyncIdentityKey(ctx context.Context, identityKey *string) error {
	identity, err := middleware.ShouldGetAuthenticatedIdentity(ctx)
	if err != nil {
		return fmt.Errorf("function may only access authenticated user: %w", err)
	}

	authenticatedKey := identity.ToDERHex()

	if is.BlankString(*identityKey) {
		*identityKey = authenticatedKey
		return nil
	}

	if *identityKey != authenticatedKey {
		return fmt.Errorf("identityKey does not match authentication")
	}

	return nil
}

func (p *RPCStorageProvider) verifyIdentityKey(ctx context.Context, identityKey string) error {
	if is.BlankString(identityKey) {
		return fmt.Errorf("identityKey does not match authentication: missing identityKey")
	}

	identity, err := middleware.ShouldGetAuthenticatedIdentity(ctx)
	if err != nil {
		return fmt.Errorf("function may only access authenticated user: %w", err)
	}

	if identity.ToDERHex() != identityKey {
		return fmt.Errorf("identityKey does not match authentication")
	}

	return nil
}

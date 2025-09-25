package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-bsv-middleware/pkg/middleware"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/is"
)

var _ wdk.WalletStorageProvider = (*RPCStorageProvider)(nil)

type RPCStorageProvider struct {
	localProvider wdk.WalletStorageProvider
	log           *slog.Logger
}

func NewRPCStorageProvider(logger *slog.Logger, localProvider wdk.WalletStorageProvider) *RPCStorageProvider {
	return &RPCStorageProvider{
		localProvider: localProvider,
		log:           logging.Child(logger, "RPCStorageProvider"),
	}
}

func (p *RPCStorageProvider) FindOrInsertUser(ctx context.Context, identityKey string) (*wdk.FindOrInsertUserResponse, error) {
	err := p.verifyIdentityKey(ctx, identityKey)
	if err != nil {
		return nil, err
	}

	return p.localProvider.FindOrInsertUser(ctx, identityKey) //nolint:wrapcheck
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

func (p *RPCStorageProvider) verifyAuthenticated(ctx context.Context) error {
	if middleware.IsNotAuthenticated(ctx) {
		return fmt.Errorf("function may only access authenticated user")
	}
	return nil
}

func (p *RPCStorageProvider) verifyAuthID(ctx context.Context, auth wdk.AuthID) error {
	return p.verifyIdentityKey(ctx, auth.IdentityKey)
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

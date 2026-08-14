package party

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// WalletParty contains information about user and storage party identities and beef party.
type WalletParty struct {
	UserParty    string
	StorageParty string
	BeefParty    *wdk.BeefParty
}

// GetKnownTxIDs merges new known transaction IDs into the wallet's known transactions and returns a list of known txIDs.
func (wp *WalletParty) GetKnownTxIDs(ctx context.Context, newKnownTxIDs ...chainhash.Hash) (primitives.TXIDHexStrings, error) {
	// Locked BeefParty wrappers: the shared Beef graph is mutated by every
	// concurrent action on this wallet.
	for _, txID := range newKnownTxIDs {
		wp.BeefParty.MergeTxidOnly(ctx, &txID)
	}

	result := wp.BeefParty.ValidateTransactions(ctx)

	hexStrings := make([]primitives.TXIDHexString, 0, len(result.Valid))
	for _, id := range result.Valid {
		hexStrings = append(hexStrings, primitives.TXIDHexString(id))
	}

	return hexStrings, nil
}

// MergeFromStorage teaches the wallet's shared beef party about a BEEF that
// storage returned.
//
// It lives here rather than in each caller because both the wallet and its
// actions need it, and duplicating it once already meant two copies drifting
// apart. BeefParty traces the merge itself, so this only adds the error context.
func MergeFromStorage(ctx context.Context, wp *WalletParty, beef primitives.BEEF) error {
	if err := wp.BeefParty.MergeBeefFromParty(ctx, wp.StorageParty, beef); err != nil {
		return fmt.Errorf("failed to merge returned BEEF from storage: %w", err)
	}
	return nil
}

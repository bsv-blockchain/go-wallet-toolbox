package party

import (
	"context"
	"fmt"
	"strings"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// VerifyReturnedTxIDOnlyBeef verify for not known TxIDOnly txs
func VerifyReturnedTxIDOnlyBeef(ctx context.Context, bp *wdk.BeefParty, beef primitives.BEEF) (primitives.BEEF, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "BeefParty-VerifyReturnedTxIDOnly",
		attribute.Int("beef.bytes", len(beef)),
	)
	defer func() { tracing.EndTracing(span, err) }()

	b, err := transaction.NewBeefFromBytes(beef)
	if err != nil {
		return nil, fmt.Errorf("failed to create beef from bytes: %w", err)
	}

	// Resolve TxIDOnly entries against the shared party graph and serialize
	// under one lock: transactions returned by FindAtomicTransactionByHash
	// reference objects inside the shared Beef, so serialization must not
	// interleave with concurrent merges.
	var bytes []byte
	err = bp.WithLock(ctx, func(partyBeef *transaction.Beef) error {
		verified, verifyErr := verifyReturnedTxIDOnly(partyBeef, b)
		if verifyErr != nil {
			return fmt.Errorf("failed to verify returned beef txid only: %w", verifyErr)
		}
		bytes, verifyErr = verified.Bytes()
		if verifyErr != nil {
			return fmt.Errorf("failed to get bytes from beef: %w", verifyErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// VerifyReturnedTxIDOnlyAtomicBEEF verify for not known TxIDOnly txs
func VerifyReturnedTxIDOnlyAtomicBEEF(ctx context.Context, bp *wdk.BeefParty, txID chainhash.Hash, beef primitives.BEEF, knownTxIDs ...primitives.TXIDHexString) (primitives.BEEF, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "BeefParty-VerifyReturnedTxIDOnlyAtomic",
		attribute.Int("beef.bytes", len(beef)),
	)
	defer func() { tracing.EndTracing(span, err) }()

	b, err := transaction.NewBeefFromBytes(beef)
	if err != nil {
		return nil, fmt.Errorf("failed to create beef from bytes: %w", err)
	}

	// See VerifyReturnedTxIDOnlyBeef for why resolution + serialization share
	// one lock over the party graph.
	var bytes []byte
	err = bp.WithLock(ctx, func(partyBeef *transaction.Beef) error {
		verified, verifyErr := verifyReturnedTxIDOnly(partyBeef, b, knownTxIDs...)
		if verifyErr != nil {
			return fmt.Errorf("failed to verify returned beef txid only: %w", verifyErr)
		}
		bytes, verifyErr = verified.AtomicBytes(&txID)
		if verifyErr != nil {
			return fmt.Errorf("failed to get bytes from beef: %w", verifyErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// verifyReturnedTxIDOnly resolves TxIDOnly entries in beef against the shared
// party graph. partyBeef must only be accessed under the BeefParty lock (see
// wdk.BeefParty.WithLock).
func verifyReturnedTxIDOnly(partyBeef, beef *transaction.Beef, knownTxIDs ...primitives.TXIDHexString) (*transaction.Beef, error) {
	for _, btx := range beef.Transactions {
		if btx.DataFormat != transaction.TxIDOnly {
			continue
		}
		tx := partyBeef.FindAtomicTransactionByHash(btx.KnownTxID)
		if tx == nil {
			return nil, fmt.Errorf("tx with only txid not found in beef party: %s", btx.KnownTxID.String())
		}

		_, err := beef.MergeTransaction(tx)
		if err != nil {
			return nil, fmt.Errorf("failed to merge transaction with only txid: %w", err)
		}
	}

	for _, btx := range beef.Transactions {
		if btx.DataFormat != transaction.TxIDOnly {
			continue
		}

		txIDHexString := primitives.TXIDHexString(btx.KnownTxID.String())
		if knownTxIDs != nil && contains(knownTxIDs, txIDHexString) {
			continue
		}

		return nil, fmt.Errorf("transaction %s returned as txidOnly but is not in the known transactions list", btx.KnownTxID.String())
	}

	// go-sdk's FindAtomicTransactionByHash walks the graph by SourceTransaction
	// pointers and silently skips an input whose source the graph holds only as a
	// bare txid, so a resolved transaction can come back with a hole in its
	// ancestry and nothing errors while building it. The hole only surfaces much
	// later, as a storage-side "provided beef is not valid" on whatever reuses
	// these bytes as an input BEEF. Catch it here, where the graph that caused it
	// is still in hand.
	if vr := beef.ValidateTransactions(); len(vr.MissingInputs) > 0 {
		return nil, fmt.Errorf("resolved beef has no ancestry for transactions: %s", strings.Join(vr.MissingInputs, ", "))
	}

	return beef, nil
}

func contains(slice []primitives.TXIDHexString, s primitives.TXIDHexString) bool {
	for _, v := range slice {
		if v.String() == s.String() {
			return true
		}
	}
	return false
}

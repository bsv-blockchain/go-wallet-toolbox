package wdk

import (
	"fmt"
	"sync"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// BeefParty represents a BEEF shared among multiple parties, tracking known transactions for each party.
//
// Concurrency: one BeefParty is shared by every action a wallet runs, and
// transaction.Beef is a plain map-backed graph with no internal locking —
// concurrent merges crash with "concurrent map iteration and map write". All
// access to the embedded Beef must therefore hold mu. Methods on BeefParty do
// this themselves; callers needing multi-step graph work (verify + serialize)
// use WithLock. Do NOT call promoted transaction.Beef methods directly from
// outside this package.
type BeefParty struct {
	*transaction.Beef

	mu      sync.Mutex
	knownTo map[string][]string
}

// DefaultMaxGraphTxs bounds the shared Beef graph. Every action merges its
// returned BEEF into the party and advertises the whole valid-txid list on the
// next request, so an unbounded graph makes each action slower than the last
// (O(n²) over a run) and leaks memory in long-running wallets. When a merge
// would grow past this cap, the graph and known-to bookkeeping reset first;
// the only cost is that the next response per tx arrives as full BEEF instead
// of TxIDOnly.
const DefaultMaxGraphTxs = 256

// NewBeefParty creates a new BeefParty instance with optional initial parties.
func NewBeefParty(parties []string) *BeefParty {
	bp := &BeefParty{
		Beef:    transaction.NewBeef(),
		knownTo: make(map[string][]string),
	}

	for _, p := range parties {
		bp.AddParty(p)
	}

	return bp
}

// resetLocked drops the accumulated graph and per-party known lists, keeping
// the party keys. Call with bp.mu held, and only BEFORE merging new data —
// callers rely on the just-merged response staying resolvable.
func (bp *BeefParty) resetLocked() {
	bp.Beef = transaction.NewBeef()
	for p := range bp.knownTo {
		bp.knownTo[p] = []string{}
	}
}

// WithLock runs fn with exclusive access to the shared Beef graph. Use it for
// multi-step reads/writes that must be atomic with respect to concurrent
// merges (e.g. resolving TxIDOnly entries and serializing the result).
func (bp *BeefParty) WithLock(fn func(beef *transaction.Beef) error) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return fn(bp.Beef)
}

// AddParty adds a new party to the BeefParty.
func (bp *BeefParty) AddParty(party string) {
	bp.mu.Lock()
	if _, ok := bp.knownTo[party]; !ok {
		bp.knownTo[party] = []string{}
	}
	bp.mu.Unlock()
}

// IsParty checks if a party is known to the BeefParty.
func (bp *BeefParty) IsParty(party string) bool {
	bp.mu.Lock()
	_, ok := bp.knownTo[party]
	bp.mu.Unlock()

	return ok
}

// GetKnownTxIDsForParty retrieves the known transaction IDs for a specific party.
func (bp *BeefParty) GetKnownTxIDsForParty(party string) ([]string, error) {
	bp.mu.Lock()
	s, ok := bp.knownTo[party]
	bp.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("unknown party: %s", party)
	}

	out := make([]string, len(s))
	copy(out, s)

	return out, nil
}

// GetTrimmedBeefForParty returns a pruned Beef containing only transactions unknown to the specified party.
func (bp *BeefParty) GetTrimmedBeefForParty(party string) (*transaction.Beef, error) {
	bp.mu.Lock()
	knownTxIDs, ok := bp.knownTo[party]
	if !ok {
		bp.mu.Unlock()
		return nil, fmt.Errorf("unknown party: %s", party)
	}
	known := make([]string, len(knownTxIDs))
	copy(known, knownTxIDs)
	prunedBeef := bp.Beef.Clone()
	bp.mu.Unlock()

	// The clone is private to this caller; trimming needs no lock.
	// TrimknownTxIDs is the go-sdk method name (lowercase 'k' is intentional in the SDK).
	prunedBeef.TrimknownTxIDs(known)

	return prunedBeef, nil
}

// AddKnownTxIDsForParty adds known transaction IDs for a specific party and merges them into the Beef.
// Duplicate txIDs for a party are ignored. The knownTo update and Beef merge are performed under a single lock.
func (bp *BeefParty) AddKnownTxIDsForParty(party string, txIDs ...string) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.addKnownTxIDsForPartyLocked(party, txIDs...)
}

func (bp *BeefParty) addKnownTxIDsForPartyLocked(party string, txIDs ...string) error {
	existing, ok := bp.knownTo[party]
	if !ok {
		existing = []string{}
	}

	seen := make(map[string]struct{}, len(existing)+len(txIDs))
	for _, id := range existing {
		seen[id] = struct{}{}
	}

	for _, txID := range txIDs {
		if _, dup := seen[txID]; !dup {
			existing = append(existing, txID)
			seen[txID] = struct{}{}
		}

		hash, err := chainhash.NewHashFromHex(txID)
		if err != nil {
			return fmt.Errorf("failed to parse string txID Hex to chainhash %s: %w", txID, err)
		}

		bp.Beef.MergeTxidOnly(hash)
	}

	bp.knownTo[party] = existing

	return nil
}

// MergeTxidOnly records a txid-only entry in the shared Beef.
// Shadows the promoted transaction.Beef method to add locking.
func (bp *BeefParty) MergeTxidOnly(txid *chainhash.Hash) *transaction.BeefTx {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.Beef.MergeTxidOnly(txid)
}

// MergeBeef merges another Beef into the shared graph.
// Shadows the promoted transaction.Beef method to add locking.
func (bp *BeefParty) MergeBeef(other *transaction.Beef) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.Beef.MergeBeef(other)
}

// Clone returns a deep copy of the shared Beef.
// Shadows the promoted transaction.Beef method to add locking.
func (bp *BeefParty) Clone() *transaction.Beef {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.Beef.Clone()
}

// ValidateTransactions validates the shared Beef's transactions.
// Shadows the promoted transaction.Beef method to add locking.
func (bp *BeefParty) ValidateTransactions() *transaction.ValidationResult {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.Beef.ValidateTransactions()
}

// MergeBeefFromParty merges a Beef from a specific party into the BeefParty and updates known transaction IDs.
func (bp *BeefParty) MergeBeefFromParty(party string, beef primitives.BEEF) error {
	b, err := transaction.NewBeefFromBytes(beef)
	if err != nil {
		return fmt.Errorf("failed to parse BEEF bytes from party %s: %w", party, err)
	}

	knownTxIDs := b.GetValidTxids()

	bp.mu.Lock()
	defer bp.mu.Unlock()

	// Bound graph growth BEFORE merging: the caller may still resolve
	// TxIDOnly entries of this response against the graph right after.
	if len(bp.Beef.Transactions) > DefaultMaxGraphTxs {
		bp.resetLocked()
	}

	if err = bp.Beef.MergeBeef(b); err != nil {
		return fmt.Errorf("failed to merge BEEF from party %s: %w", party, err)
	}

	if err = bp.addKnownTxIDsForPartyLocked(party, knownTxIDs...); err != nil {
		return fmt.Errorf("failed to add known txIDs from party %s: %w", party, err)
	}

	return nil
}

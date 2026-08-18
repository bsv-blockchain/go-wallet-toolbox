package wdk

import (
	"context"
	"fmt"
	"sync"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/sdkbeef"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
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

	// leases counts the actions currently inside an advertise->resolve window.
	// While any is open the graph must not be dropped; see Lease.
	leases int
	// resetPending records a bound that was reached while leases were open, so
	// the last release performs the reset the bound asked for.
	resetPending bool
}

// DefaultMaxGraphTxs bounds the shared Beef graph. Every action merges its
// returned BEEF into the party and advertises the whole valid-txid list on the
// next request, so an unbounded graph makes each action slower than the last
// (O(n²) over a run) and leaks memory in long-running wallets. When a merge
// would grow past this cap, the graph and known-to bookkeeping reset first;
// the only cost is that the next response per tx arrives as full BEEF instead
// of TxIDOnly.
const DefaultMaxGraphTxs = 256

// DefaultMaxGraphBUMPs bounds the shared graph's merkle proofs. Transaction
// count alone does not bound the work: merging a proof is compared against the
// proofs already held, so a graph carrying many of them makes every later merge
// more expensive even while the transaction count stays under its own cap.
// Proofs for the same block combine into one, so reaching this many means the
// wallet has genuinely spanned that many blocks and the graph is stale enough
// to drop.
const DefaultMaxGraphBUMPs = 64

// EmergencyResetFactor is how far past its bounds the graph may grow while
// actions hold leases before it is reset anyway.
//
// A deferred reset is correct but not sufficient on its own: under sustained
// concurrency the leases can overlap forever, the graph never reaches
// quiescence, and the unbounded growth #996 set out to fix comes back. So a
// hard ceiling still resets, and the stranded replies degrade instead - callers
// that cannot resolve a bare txid fall back to the BEEF storage sent (see
// wallet CreateAction) rather than failing an action whose transaction is
// already broadcast.
const EmergencyResetFactor = 4

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
//
// Waiting and holding are traced separately. One wallet shares one graph behind
// one mutex, so a slow action is either doing too much work or queueing behind
// something else, and only splitting the two tells you which.
func (bp *BeefParty) WithLock(ctx context.Context, fn func(beef *transaction.Beef) error) error {
	// Deliberately not rebinding ctx: the two spans are siblings under the
	// caller, not one inside the other. Starting the hold span from the wait
	// span's context would nest it inside a span that has already ended, which
	// is precisely the comparison this split exists to make readable.
	_, waitSpan := tracing.StartTracing(ctx, "BeefParty-LockWait")
	bp.mu.Lock()
	tracing.EndTracing(waitSpan, nil)
	defer bp.mu.Unlock()

	var err error
	_, holdSpan := tracing.StartTracing(ctx, "BeefParty-LockHeld", bp.graphAttrs()...)
	defer func() { tracing.EndTracing(holdSpan, err) }()

	err = fn(bp.Beef)
	return err
}

// graphAttrs describes the current graph. Call with bp.mu held.
func (bp *BeefParty) graphAttrs() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("party.graph_txs", len(bp.Transactions)),
		attribute.Int("party.graph_bumps", len(bp.BUMPs)),
	}
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
func (bp *BeefParty) GetKnownTxIDsForParty(ctx context.Context, party string) ([]string, error) {
	var err error
	_, span := tracing.StartTracing(ctx, "BeefParty-GetKnownTxIDsForParty",
		attribute.String("party", party),
	)
	defer func() { tracing.EndTracing(span, err) }()

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
func (bp *BeefParty) GetTrimmedBeefForParty(ctx context.Context, party string) (*transaction.Beef, error) {
	var err error
	_, span := tracing.StartTracing(ctx, "BeefParty-GetTrimmedBeefForParty",
		attribute.String("party", party),
	)
	defer func() { tracing.EndTracing(span, err) }()

	bp.mu.Lock()
	knownTxIDs, ok := bp.knownTo[party]
	if !ok {
		bp.mu.Unlock()
		err = fmt.Errorf("unknown party: %s", party)
		return nil, err
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
func (bp *BeefParty) AddKnownTxIDsForParty(ctx context.Context, party string, txIDs ...string) error {
	var err error
	_, span := tracing.StartTracing(ctx, "BeefParty-AddKnownTxIDsForParty",
		attribute.String("party", party),
		attribute.Int("txids", len(txIDs)),
	)
	defer func() { tracing.EndTracing(span, err) }()

	bp.mu.Lock()
	defer bp.mu.Unlock()
	err = bp.addKnownTxIDsForPartyLocked(party, txIDs...)
	return err
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
func (bp *BeefParty) MergeTxidOnly(ctx context.Context, txid *chainhash.Hash) *transaction.BeefTx {
	_, span := tracing.StartTracing(ctx, "BeefParty-MergeTxidOnly")
	defer func() { tracing.EndTracing(span, nil) }()

	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.Beef.MergeTxidOnly(txid)
}

// MergeBeef merges another Beef into the shared graph.
// Shadows the promoted transaction.Beef method to add locking.
func (bp *BeefParty) MergeBeef(ctx context.Context, other *transaction.Beef) error {
	var err error
	_, span := tracing.StartTracing(ctx, "BeefParty-MergeBeef",
		attribute.Int("other.txs", len(other.Transactions)),
		attribute.Int("other.bumps", len(other.BUMPs)),
	)
	defer func() { tracing.EndTracing(span, err) }()

	bp.mu.Lock()
	defer bp.mu.Unlock()
	span.SetAttributes(bp.graphAttrs()...)
	err = bp.Beef.MergeBeef(other)
	return err
}

// Clone returns a deep copy of the shared Beef.
// Shadows the promoted transaction.Beef method to add locking.
func (bp *BeefParty) Clone(ctx context.Context) *transaction.Beef {
	_, span := tracing.StartTracing(ctx, "BeefParty-Clone")
	defer func() { tracing.EndTracing(span, nil) }()

	bp.mu.Lock()
	defer bp.mu.Unlock()
	span.SetAttributes(bp.graphAttrs()...)
	return bp.Beef.Clone()
}

// ValidateTransactions validates the shared Beef's transactions.
// Shadows the promoted transaction.Beef method to add locking.
//
// This is where the graph is bounded, because this is where a caller decides
// what to advertise as known. Dropping transactions at any later point would
// strand a response already asking for them; see PruneIfOversized.
func (bp *BeefParty) ValidateTransactions(ctx context.Context) *transaction.ValidationResult {
	_, span := tracing.StartTracing(ctx, "BeefParty-ValidateTransactions")
	defer func() { tracing.EndTracing(span, nil) }()

	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.pruneIfOversizedLocked()
	span.SetAttributes(bp.graphAttrs()...)
	return bp.Beef.ValidateTransactions()
}

// PruneIfOversized drops the accumulated graph once it grows past its bounds.
//
// Call it once an action has finished with the graph — after the reply has been
// resolved and serialized — and never between advertising and resolving.
// Storage answers with bare txids for whatever was advertised, and those can
// only be resolved from the graph that produced them, so pruning in between
// leaves the wallet unable to produce transactions it just claimed to hold.
//
// Every action that merges a reply must call this, including one whose caller
// supplied its own known-txid list and so never advertised: the merge grows the
// graph either way, and this is the only thing that bounds it.
func (bp *BeefParty) PruneIfOversized(ctx context.Context) {
	_, span := tracing.StartTracing(ctx, "BeefParty-PruneIfOversized")
	defer func() { tracing.EndTracing(span, nil) }()

	bp.mu.Lock()
	defer bp.mu.Unlock()
	span.SetAttributes(bp.graphAttrs()...)
	bp.pruneIfOversizedLocked()
}

func (bp *BeefParty) pruneIfOversizedLocked() {
	if len(bp.Transactions) <= DefaultMaxGraphTxs && len(bp.BUMPs) <= DefaultMaxGraphBUMPs {
		return
	}

	beyondCeiling := len(bp.Transactions) > DefaultMaxGraphTxs*EmergencyResetFactor ||
		len(bp.BUMPs) > DefaultMaxGraphBUMPs*EmergencyResetFactor

	if bp.leases > 0 && !beyondCeiling {
		// An action is between advertising its known txids and resolving the
		// reply built from them. Dropping the graph now would strand that reply,
		// so the reset waits for the last lease to be released.
		bp.resetPending = true
		return
	}

	bp.resetLocked()
	bp.resetPending = false
}

// Lease marks the start of an advertise->resolve window and returns the release
// to call when the window closes. Release is idempotent.
//
// Storage answers with bare txids for whatever the wallet advertised, and those
// can only be resolved from the graph that produced them. A bound reached by a
// concurrent action in between would drop that graph and leave this action
// unable to produce transactions it just claimed to hold, so while any lease is
// open the bound is recorded and applied by the last release instead.
func (bp *BeefParty) Lease(ctx context.Context) func() {
	_, span := tracing.StartTracing(ctx, "BeefParty-Lease")
	defer func() { tracing.EndTracing(span, nil) }()

	bp.mu.Lock()
	bp.leases++
	bp.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			bp.mu.Lock()
			defer bp.mu.Unlock()

			bp.leases--
			if bp.leases == 0 && bp.resetPending {
				bp.resetLocked()
				bp.resetPending = false
			}
		})
	}
}

// MergeBeefFromParty merges a Beef from a specific party into the BeefParty and updates known transaction IDs.
func (bp *BeefParty) MergeBeefFromParty(ctx context.Context, party string, beef primitives.BEEF) error {
	var err error
	_, span := tracing.StartTracing(ctx, "BeefParty-MergeBeefFromParty",
		attribute.String("party", party),
		attribute.Int("beef.bytes", len(beef)),
	)
	defer func() { tracing.EndTracing(span, err) }()

	b, err := sdkbeef.ParseBytes(beef)
	if err != nil {
		return fmt.Errorf("failed to parse BEEF bytes from party %s: %w", party, err)
	}

	knownTxIDs := b.GetValidTxids()

	bp.mu.Lock()
	defer bp.mu.Unlock()

	if err = bp.Beef.MergeBeef(b); err != nil {
		return fmt.Errorf("failed to merge BEEF from party %s: %w", party, err)
	}

	if err = bp.addKnownTxIDsForPartyLocked(party, knownTxIDs...); err != nil {
		return fmt.Errorf("failed to add known txIDs from party %s: %w", party, err)
	}

	return nil
}

package wdk

import (
	"fmt"
	"sync"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// BeefParty represents a BEEF shared among multiple parties, tracking known transactions for each party.
type BeefParty struct {
	*transaction.Beef

	mu      sync.RWMutex
	knownTo map[string][]string
}

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

// AddParty adds a new party to the BeefParty.
func (bp *BeefParty) AddParty(party string) {
	bp.mu.Lock()
	bp.knownTo[party] = []string{}
	bp.mu.Unlock()
}

// IsParty checks if a party is known to the BeefParty.
func (bp *BeefParty) IsParty(party string) bool {
	bp.mu.RLock()
	_, ok := bp.knownTo[party]
	bp.mu.RUnlock()

	return ok
}

// GetKnownTxIDsForParty retrieves the known transaction IDs for a specific party.
func (bp *BeefParty) GetKnownTxIDsForParty(party string) ([]string, error) {
	bp.mu.RLock()
	s, ok := bp.knownTo[party]
	bp.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown party: %s", party)
	}

	out := make([]string, len(s))
	copy(out, s)

	return out, nil
}

// GetTrimmedBeefForParty returns a pruned Beef containing only transactions unknown to the specified party.
func (bp *BeefParty) GetTrimmedBeefForParty(party string) (*transaction.Beef, error) {
	knownTxIDs, err := bp.GetKnownTxIDsForParty(party)
	if err != nil {
		return nil, err
	}

	prunedBeef := bp.Clone()
	prunedBeef.TrimknownTxIDs(knownTxIDs)

	return prunedBeef, nil
}

// AddKnownTxIDsForParty adds known transaction IDs for a specific party and merges them into the Beef.
func (bp *BeefParty) AddKnownTxIDsForParty(party string, txIDs ...string) error {
	if !bp.IsParty(party) {
		bp.AddParty(party)
	}

	bp.addTxIDsForParty(party, txIDs)

	for _, txID := range txIDs {
		hash, err := chainhash.NewHashFromHex(txID)
		if err != nil {
			return fmt.Errorf("failed to parse string txID Hex to chainhash %s: %w", txID, err)
		}

		bp.MergeTxidOnly(hash)
	}

	return nil
}

func (bp *BeefParty) addTxIDsForParty(party string, txIDs []string) {
	bp.mu.Lock()
	bp.knownTo[party] = append(bp.knownTo[party], txIDs...)
	bp.mu.Unlock()
}

// MergeBeefFromParty merges a Beef from a specific party into the BeefParty and updates known transaction IDs.
func (bp *BeefParty) MergeBeefFromParty(party string, beef primitives.BEEF) error {
	b, err := transaction.NewBeefFromBytes(beef)
	if err != nil {
		return fmt.Errorf("failed to parse BEEF bytes from party %s: %w", party, err)
	}

	knownTxIDs := b.GetValidTxids()

	err = bp.MergeBeef(b)
	if err != nil {
		return fmt.Errorf("failed to merge BEEF from party %s: %w", party, err)
	}

	err = bp.AddKnownTxIDsForParty(party, knownTxIDs...)
	if err != nil {
		return fmt.Errorf("failed to add known txIDs from party %s: %w", party, err)
	}

	return nil
}

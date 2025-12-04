package wdk

import (
	"fmt"
	"sync"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

type BeefParty struct {
	*transaction.Beef

	mu      sync.RWMutex
	knownTo map[string][]string
}

func NewBeefParty(parties []string) *BeefParty {
	bp := &BeefParty{
		Beef:    transaction.NewBeef(),
		knownTo: make(map[string][]string),
	}

	if parties != nil {
		for _, p := range parties {
			bp.AddParty(p)
		}
	}

	return bp
}

func (bp *BeefParty) AddParty(party string) {
	bp.mu.Lock()
	bp.knownTo[party] = []string{}
	bp.mu.Unlock()
}

func (bp *BeefParty) IsParty(party string) bool {
	bp.mu.RLock()
	_, ok := bp.knownTo[party]
	bp.mu.RUnlock()

	return ok
}

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

func (bp *BeefParty) GetTrimmedBeefForParty(party string) (*transaction.Beef, error) {
	knownTxIDs, err := bp.GetKnownTxIDsForParty(party)
	if err != nil {
		return nil, err
	}

	prunedBeef := bp.Clone()
	prunedBeef.TrimknownTxIDs(knownTxIDs)

	return prunedBeef, nil
}

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

func (bp *BeefParty) MergeBeefFromParty(party string, beef primitives.BEEF) error {
	b, err := transaction.NewBeefFromBytes(beef)
	if err != nil {
		return err
	}

	knownTxIDs := b.GetValidTxids()

	err = bp.MergeBeef(b)
	if err != nil {
		return err
	}

	err = bp.AddKnownTxIDsForParty(party, knownTxIDs...)
	if err != nil {
		return err
	}

	return nil
}

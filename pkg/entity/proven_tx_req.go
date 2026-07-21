package entity

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// ReqHistoryNote implements the flattened extras
type ReqHistoryNote map[string]any

// What returns the event tag of this history note.
func (n ReqHistoryNote) What() string {
	if v, ok := n["what"].(string); ok {
		return v
	}
	return ""
}

// When returns the parsed ISO timestamp of this history note, if present.
func (n ReqHistoryNote) When() *time.Time {
	if v, ok := n["when"].(string); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			return &t
		}
	}
	return nil
}

// ProvenTxReqHistory represents the JSON serialized history notes for a ProvenTxReq.
type ProvenTxReqHistory struct {
	Notes []ReqHistoryNote `json:"notes,omitempty"`
}

// AddHistoryNote appends a note, optionally deduplicating by 'what' and 'when'.
// It maintains union-sort order by 'when'.
func (h *ProvenTxReqHistory) AddHistoryNote(note ReqHistoryNote, noDupes bool) {
	if h.Notes == nil {
		h.Notes = []ReqHistoryNote{}
	}
	if noDupes {
		what := note.What()
		when := note.When()
		for i, existing := range h.Notes {
			if existing.What() == what {
				var sameTime bool
				existingWhen := existing.When()
				if when == nil && existingWhen == nil {
					sameTime = true
				} else if when != nil && existingWhen != nil && when.Equal(*existingWhen) {
					sameTime = true
				}
				if sameTime {
					// Merge existing note with new note (union-sort / mergeExisting)
					for k, v := range note {
						h.Notes[i][k] = v
					}
					return
				}
			}
		}
	}
	h.Notes = append(h.Notes, note)

	// Sort by when
	sort.SliceStable(h.Notes, func(i, j int) bool {
		t1 := h.Notes[i].When()
		t2 := h.Notes[j].When()
		if t1 == nil && t2 == nil {
			return false
		}
		if t1 == nil {
			return true // Put missing time notes at the end or beginning? Let's put at the end
		}
		if t2 == nil {
			return false
		}
		return t1.Before(*t2)
	})
}

// HistorySince filters notes by when > date.
func (h *ProvenTxReqHistory) HistorySince(date time.Time) []ReqHistoryNote {
	var result []ReqHistoryNote
	for _, n := range h.Notes {
		if t := n.When(); t != nil && t.After(date) {
			result = append(result, n)
		}
	}
	return result
}

// HistoryPretty returns a human-readable render of the history.
func (h *ProvenTxReqHistory) HistoryPretty(since *time.Time, indent int) string {
	var lines []string
	prefix := strings.Repeat(" ", indent)
	notes := h.Notes
	if since != nil {
		notes = h.HistorySince(*since)
	}
	for _, n := range notes {
		b, err := json.Marshal(n)
		if err == nil {
			lines = append(lines, prefix+string(b))
		}
	}
	return strings.Join(lines, "\n")
}

// GetHistorySummary derives flags from notes[].what.
func (h *ProvenTxReqHistory) GetHistorySummary() map[string]int {
	summary := make(map[string]int)
	for _, n := range h.Notes {
		summary[n.What()]++
	}
	return summary
}

// ProvenTxReqNotify represents the JSON serialized notification IDs for a ProvenTxReq.
type ProvenTxReqNotify struct {
	TransactionIDs []uint `json:"transactionIds,omitempty"`
}

// ProvenTxReq represents a request for a proven transaction.
type ProvenTxReq struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time

	ProvenTxID          *uint
	Status              wdk.ProvenTxReqStatus
	Attempts            uint64
	Notified            bool
	TxID                string
	Batch               *string
	History             ProvenTxReqHistory
	Notify              ProvenTxReqNotify
	RawTx               []byte
	InputBEEF           []byte
	WasBroadcast        bool
	RebroadcastAttempts uint64
}

// ProvenTxReqReadSpecification defines criteria for querying proven tx requests.
type ProvenTxReqReadSpecification struct {
	ID                  *uint
	TxID                *string
	TxIDs               []string
	IncludeHistoryNotes bool
	Status              *Comparable[wdk.ProvenTxReqStatus]
	Attempts            *Comparable[uint64]
	Notified            *Comparable[bool]
	Batch               *Comparable[string]
}

// TxHistoryNote represents a single transaction event note, combining general event metadata with a transaction ID.
type TxHistoryNote struct {
	wdk.HistoryNote
	TxID string
}

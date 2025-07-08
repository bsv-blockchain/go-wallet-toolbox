package utils

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// ConvertNotes converts a slice of strings into wdk.Notes, assigning the current time to each note.
func ConvertNotes(notes []string) wdk.Notes {
	converted := make(wdk.Notes, len(notes))
	for i, note := range notes {
		now := time.Now()
		converted[i] = wdk.ReqHistoryNote{
			When: &now,
			What: note,
		}
	}
	return converted
}

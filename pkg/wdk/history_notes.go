package wdk

import (
	"fmt"
	"io"
	"time"
)

const (
	whatAttr   = "what"
	userIDAttr = "user_id"
	whenAttr   = "when"
)

// HistoryNote represents a transaction event with metadata including time, user information, and event attributes.
// It's an equivalent to the HistoryNote in wdk.ProvenTxReq
type HistoryNote struct {
	When time.Time

	UserID *int

	What       string
	Attributes map[string]any
}

func (n *HistoryNote) ToMap() map[string]any {
	all := make(map[string]any, len(n.Attributes)+4)
	all[whatAttr] = n.What
	all[userIDAttr] = n.UserID
	all[whenAttr] = n.When

	return all
}

func (n *HistoryNote) PrettyPrint(writer io.Writer) error {
	for k, v := range n.ToMap() {
		if _, err := writer.Write([]byte(fmt.Sprintf("%s: %v\n", k, v))); err != nil {
			return fmt.Errorf("error writing history note attribute %s: %w", k, err)
		}
	}
	return nil
}

func (n *HistoryNote) AsList() HistoryNotes {
	return HistoryNotes{n}
}

type HistoryNotes []*HistoryNote

func (h HistoryNotes) PrettyPrint(writer io.Writer) error {
	for _, note := range h {
		if err := note.PrettyPrint(writer); err != nil {
			return err
		}
		if _, err := writer.Write([]byte("\n\n")); err != nil {
			return fmt.Errorf("error writing separator after history note: %w", err)
		}
	}
	return nil
}

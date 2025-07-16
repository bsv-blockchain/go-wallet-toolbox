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

// ToMap returns a map representation of the HistoryNote, including its core attributes and event information.
func (n *HistoryNote) ToMap() map[string]any {
	all := make(map[string]any, len(n.Attributes)+4)
	all[whatAttr] = n.What
	all[userIDAttr] = n.UserID
	all[whenAttr] = n.When

	return all
}

// PrettyPrint writes the HistoryNote fields and attributes to the specified writer in a human-readable format.
// Returns an error if writing to the writer fails for any attribute or field.
func (n *HistoryNote) PrettyPrint(writer io.Writer) error {
	for k, v := range n.ToMap() {
		if _, err := writer.Write([]byte(fmt.Sprintf("%s: %v\n", k, v))); err != nil {
			return fmt.Errorf("error writing history note attribute %s: %w", k, err)
		}
	}
	return nil
}

// AsList returns a HistoryNotes slice containing the receiver HistoryNote as its only element.
func (n *HistoryNote) AsList() HistoryNotes {
	return HistoryNotes{n}
}

// HistoryNotes is a slice of pointers to HistoryNote representing a collection of transaction event logs.
type HistoryNotes []*HistoryNote

// PrettyPrint writes all history notes in a human-readable format to the provided writer, separated by double newlines.
// Returns an error if writing any note or separator fails.
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

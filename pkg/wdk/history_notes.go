package wdk

import (
	"fmt"
	"io"
	"time"

	"gopkg.in/yaml.v3"
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
	result := make(map[string]any, len(n.Attributes)+4)

	for k, v := range n.Attributes {
		result[k] = v
	}

	result[whatAttr] = n.What
	result[userIDAttr] = n.UserID
	result[whenAttr] = n.When

	return result
}

// PrettyPrint writes the HistoryNote fields and attributes to the specified writer in a human-readable format.
// Returns an error if writing to the writer fails for any attribute or field.
func (n *HistoryNote) PrettyPrint(writer io.Writer) error {
	err := yaml.NewEncoder(writer).Encode(n.ToMap())
	if err != nil {
		return fmt.Errorf("error writing history note: %w", err)
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
	allNotes := make([]map[string]any, len(h))
	for i, note := range h {
		allNotes[i] = note.ToMap()
	}

	err := yaml.NewEncoder(writer).Encode(allNotes)
	if err != nil {
		return fmt.Errorf("error writing history notes: %w", err)
	}
	return nil
}

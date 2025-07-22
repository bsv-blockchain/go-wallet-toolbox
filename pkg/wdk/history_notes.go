package wdk

import (
	"encoding/json"
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
	When time.Time `json:"when"`

	UserID *int `json:"user_id,omitempty"`

	What       string `json:"what"`
	Attributes map[string]any
}

// MarshalJSON serializes the HistoryNote into JSON, including "when", "what", "user_id" (if not nil), and all attributes.
// NOTE: The receiver must be "by value" because this is required by the json.Marshaler interface.
func (n HistoryNote) MarshalJSON() ([]byte, error) {
	count := len(n.Attributes) + 2 // +2 for "when" and "what"
	if n.UserID != nil {
		count++
	}

	data := make(map[string]any, count)
	for key, value := range n.Attributes {
		data[key] = value
	}
	data[whenAttr] = n.When
	data[whatAttr] = n.What
	if n.UserID != nil {
		data[userIDAttr] = n.UserID
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal history note: %w", err)
	}
	return encoded, nil
}

// UnmarshalJSON populates the HistoryNote from JSON, separating core fields from additional attributes.
// It extracts "when", "user_id", and "what" fields, and assigns any extra fields to the Attributes map.
// Returns an error if the JSON is invalid or required fields are missing.
func (n *HistoryNote) UnmarshalJSON(data []byte) error {
	type alias HistoryNote // use a type alias to avoid an infinite recursion loop.
	var aux alias          // This allows us to use the default json.Unmarshal logic for the known fields.

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal history note known fields: %w", err)
	}

	n.When = aux.When
	n.UserID = aux.UserID
	n.What = aux.What

	var rawData map[string]any
	if err := json.Unmarshal(data, &rawData); err != nil {
		return fmt.Errorf("failed to unmarshal history note attributes: %w", err)
	}

	delete(rawData, whenAttr)
	delete(rawData, userIDAttr)
	delete(rawData, whatAttr)

	n.Attributes = rawData

	return nil
}

// ToMap returns a map representation of the HistoryNote, including its core attributes and event information.
func (n *HistoryNote) ToMap() map[string]any {
	const coreAttributesCount = 3
	result := make(map[string]any, len(n.Attributes)+coreAttributesCount)

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

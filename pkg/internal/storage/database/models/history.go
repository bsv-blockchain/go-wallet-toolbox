package models

import (
	"encoding/json"
	"fmt"
	"time"
)

type HistoryModel struct {
	Notes []HistoryNote `json:"notes"`
}

type HistoryNote struct {
	When  time.Time
	What  string
	Attrs map[string]any
}

func (h HistoryNote) MarshalJSON() ([]byte, error) {
	result := make(map[string]any)

	result["what"] = h.What
	result["when"] = h.When

	for k, v := range h.Attrs {
		result[k] = v
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal HistoryNote: %w", err)
	}
	return jsonData, nil
}

func (h *HistoryNote) UnmarshalJSON(data []byte) error {
	if h == nil {
		return fmt.Errorf("HistoryNote: nil pointer dereference")
	}
	var partial struct {
		What string    `json:"what"`
		When time.Time `json:"when"`
	}
	if err := json.Unmarshal(data, &partial); err != nil {
		return fmt.Errorf("failed to unmarshal HistoryNote: %w", err)
	}
	h.What = partial.What
	h.When = partial.When

	var attrs map[string]any
	if err := json.Unmarshal(data, &attrs); err != nil {
		return fmt.Errorf("failed to unmarshal HistoryNote: %w", err)
	}

	delete(attrs, "what")
	delete(attrs, "when")

	h.Attrs = attrs
	return nil
}

package wdk

import "time"

// TableOutputTagMap represents the mapping between an output tag and a transaction in a table.
type TableOutputTagMap struct {
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	OutputTagID uint      `json:"outputTagId"`
	OutputID    uint      `json:"outputId"`
	IsDeleted   bool      `json:"isDeleted"`
}

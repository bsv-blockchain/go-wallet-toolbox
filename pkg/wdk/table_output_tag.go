package wdk

import "time"

// TableOutputTag represents metadata for an output tag entry in the table, including audit and identification fields.
type TableOutputTag struct {
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	OutputTagID uint      `json:"outputTagId"`
	UserID      int       `json:"userId"`
	Tag         string    `json:"tag"`
	IsDeleted   bool      `json:"isDeleted"`
}

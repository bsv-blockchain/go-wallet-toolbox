package entity

import (
	"time"

	"gorm.io/datatypes"
)

// TxNotes represents a single transaction event note in the read model.
type TxNotes struct {
	ID        uint
	CreatedAt time.Time
	DeletedAt *time.Time

	TxID   string
	UserID *int

	What       string
	Attributes datatypes.JSONMap
}

// TxNoteReadSpecification defines optional filters for reading TxNotes.
type TxNoteReadSpecification struct {
	TxID      *string
	UserID    *Comparable[int]
	What      *Comparable[string]
	CreatedAt *Comparable[time.Time]
	DeletedAt *Comparable[time.Time]
}

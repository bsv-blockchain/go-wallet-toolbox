package entity

import (
	"time"
)

type Label struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	Name   string
	UserID int
}


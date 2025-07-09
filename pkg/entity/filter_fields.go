package entity

// SinceField represents fields on which time-based filtering can be applied, such as created or updated timestamps.
type SinceField int

// Possible values for SinceField, indicating which timestamp field to use for filtering records based on creation or update time.
const (
	SinceFieldCreatedAt SinceField = iota
	SinceFieldUpdatedAt
)

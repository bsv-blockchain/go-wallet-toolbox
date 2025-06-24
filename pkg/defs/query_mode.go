package defs

import "fmt"

// QueryMode represents the mode used to filter or combine query parameters, such as 'any' or 'all' logic.
type QueryMode string

const (
	QueryModeAny QueryMode = "any" // QueryModeAny is used to indicate that any of the provided query parameters can match.
	QueryModeAll QueryMode = "all" // QueryModeAll is used to indicate that all of the provided query parameters must match.
)

// ParseQueryMode parses a string into a QueryMode, matching values case-insensitively to "any" or "all".
// Returns an error if the input is not a valid QueryMode value.
func ParseQueryMode(str string) (QueryMode, error) {
	return parseEnumCaseInsensitive(str, QueryModeAll, QueryModeAny)
}

// Value returns the QueryMode value of the receiver, defaulting to QueryModeAny if unset or nil.
// It parses the value case-insensitively and returns an error if invalid.
func (q *QueryMode) Value() (QueryMode, error) {
	if q == nil || *q == "" {
		return QueryModeAny, nil
	}
	return ParseQueryMode(string(*q))
}

// Validate checks if the QueryMode receiver contains a valid value, returning an error if not valid.
func (q *QueryMode) Validate() error {
	_, err := q.Value()
	if err != nil {
		return fmt.Errorf("invalid query mode: %w", err)
	}

	return nil
}

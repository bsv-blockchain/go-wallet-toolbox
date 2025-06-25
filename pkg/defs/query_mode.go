package defs

import "fmt"

// QueryMode represents the mode used to filter or combine query parameters, such as 'any' or 'all' logic.
type QueryMode string

const (
	// QueryModeAny is used to indicate that any of the provided query parameters can match.
	QueryModeAny QueryMode = "any"

	// QueryModeAll is used to indicate that all of the provided query parameters must match.
	QueryModeAll QueryMode = "all"
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

// MustGetValue returns the validated QueryMode value or panics if the value is invalid.
// It guarantees a valid QueryMode result and triggers a panic on error from Value.
func (q *QueryMode) MustGetValue() QueryMode {
	value, err := q.Value()
	if err != nil {
		panic(fmt.Sprintf("invalid query mode: %s", *q))
	}
	return value
}

// Validate checks if the QueryMode receiver contains a valid value, returning an error if not valid.
func (q *QueryMode) Validate() error {
	_, err := q.Value()
	if err != nil {
		return fmt.Errorf("invalid query mode: %w", err)
	}

	return nil
}

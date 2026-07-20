// Package brc114 implements BRC-114 action time label helpers used by listActions.
//
// Time control labels embedded in the labels query parameter:
//   - "action time from <unix-ms>" — inclusive lower bound on action created_at
//   - "action time to <unix-ms>"   — exclusive upper bound on action created_at
//
// When a time filter is active and includeLabels is true, responses may also
// contain computed labels of the form "action time <unix-ms>" for each action.
package brc114

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// ActionTimeFromPrefix is the BRC-114 inclusive lower-bound control label prefix.
	ActionTimeFromPrefix = "action time from "
	// ActionTimeToPrefix is the BRC-114 exclusive upper-bound control label prefix.
	ActionTimeToPrefix = "action time to "
	// ActionTimeLabelPrefix is the prefix of computed action-time response labels.
	ActionTimeLabelPrefix = "action time "

	// MaxSafeInteger is JavaScript Number.MAX_SAFE_INTEGER (2^53 - 1).
	// Timestamps above this are rejected for TS parity.
	MaxSafeInteger int64 = 9007199254740991
)

// ParsedActionTimeLabels holds the result of stripping BRC-114 time control labels.
type ParsedActionTimeLabels struct {
	// From is the inclusive lower bound in unix milliseconds, if present.
	From *int64
	// To is the exclusive upper bound in unix milliseconds, if present.
	To *int64
	// TimeFilterRequested is true when at least one time control label was present.
	TimeFilterRequested bool
	// RemainingLabels are the original labels with time control labels removed.
	// Relative order of non-control labels is preserved.
	RemainingLabels []string
}

// ParseActionTimeLabels extracts BRC-114 time control labels from a labels list.
// Time control labels are removed from RemainingLabels so they are not treated as
// ordinary DB label filters. Invalid control labels return an error.
func ParseActionTimeLabels(labels []string) (ParsedActionTimeLabels, error) {
	var state actionTimeParseState
	remaining := make([]string, 0, len(labels))

	for _, label := range labels {
		handled, err := state.consume(label)
		if err != nil {
			return ParsedActionTimeLabels{}, err
		}
		if !handled {
			remaining = append(remaining, label)
		}
	}

	if err := state.validateRange(); err != nil {
		return ParsedActionTimeLabels{}, err
	}

	return ParsedActionTimeLabels{
		From:                state.from,
		To:                  state.to,
		TimeFilterRequested: state.timeFilterRequested,
		RemainingLabels:     remaining,
	}, nil
}

// actionTimeParseState accumulates BRC-114 from/to control labels while scanning.
type actionTimeParseState struct {
	from                *int64
	to                  *int64
	timeFilterRequested bool
}

// consume tries to treat label as a time control. handled is true when the label
// was a control (and must not appear in RemainingLabels).
func (s *actionTimeParseState) consume(label string) (handled bool, err error) {
	if strings.HasPrefix(label, ActionTimeFromPrefix) {
		s.timeFilterRequested = true
		if s.from != nil {
			return true, fmt.Errorf("labels: valid. Duplicate action time from label")
		}
		n, err := parseUnixMillis(label[len(ActionTimeFromPrefix):], "from")
		if err != nil {
			return true, err
		}
		s.from = &n
		return true, nil
	}

	if strings.HasPrefix(label, ActionTimeToPrefix) {
		s.timeFilterRequested = true
		if s.to != nil {
			return true, fmt.Errorf("labels: valid. Duplicate action time to label")
		}
		n, err := parseUnixMillis(label[len(ActionTimeToPrefix):], "to")
		if err != nil {
			return true, err
		}
		s.to = &n
		return true, nil
	}

	return false, nil
}

func (s *actionTimeParseState) validateRange() error {
	if s.from != nil && s.to != nil && *s.from >= *s.to {
		return fmt.Errorf("labels: valid. action time from must be less than action time to")
	}
	return nil
}

// MakeActionTimeLabel builds the computed response label for an action's creation time.
func MakeActionTimeLabel(unixMillis int64) string {
	return fmt.Sprintf("%s%d", ActionTimeLabelPrefix, unixMillis)
}

// FromMillis converts a unix-millisecond timestamp to a time.Time in UTC.
func FromMillis(unixMillis int64) time.Time {
	return time.UnixMilli(unixMillis).UTC()
}

func parseUnixMillis(v, kind string) (int64, error) {
	if v == "" || !isAllDigits(v) {
		return 0, fmt.Errorf("labels: valid. Invalid action time %s timestamp value", kind)
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 || n > MaxSafeInteger {
		return 0, fmt.Errorf("labels: valid. Invalid action time %s timestamp value", kind)
	}
	return n, nil
}

func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

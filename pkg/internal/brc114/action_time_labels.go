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
	var from, to *int64
	remaining := make([]string, 0, len(labels))
	timeFilterRequested := false

	for _, label := range labels {
		if strings.HasPrefix(label, ActionTimeFromPrefix) {
			timeFilterRequested = true
			if from != nil {
				return ParsedActionTimeLabels{}, fmt.Errorf("labels: valid. Duplicate action time from label")
			}
			n, err := parseUnixMillis(label[len(ActionTimeFromPrefix):], "from")
			if err != nil {
				return ParsedActionTimeLabels{}, err
			}
			from = &n
			continue
		}

		if strings.HasPrefix(label, ActionTimeToPrefix) {
			timeFilterRequested = true
			if to != nil {
				return ParsedActionTimeLabels{}, fmt.Errorf("labels: valid. Duplicate action time to label")
			}
			n, err := parseUnixMillis(label[len(ActionTimeToPrefix):], "to")
			if err != nil {
				return ParsedActionTimeLabels{}, err
			}
			to = &n
			continue
		}

		remaining = append(remaining, label)
	}

	if from != nil && to != nil && *from >= *to {
		return ParsedActionTimeLabels{}, fmt.Errorf("labels: valid. action time from must be less than action time to")
	}

	return ParsedActionTimeLabels{
		From:                from,
		To:                  to,
		TimeFilterRequested: timeFilterRequested,
		RemainingLabels:     remaining,
	}, nil
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

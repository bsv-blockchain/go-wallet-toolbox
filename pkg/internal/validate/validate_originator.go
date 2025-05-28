package validate

import (
	"fmt"
	"strings"

	"github.com/go-softwarelab/common/pkg/is"
)

func Originator(originator string) error {
	if originator == "" {
		return nil
	}

	originator = normalizeOriginator(originator)
	if !is.Between(len(originator), 1, 250) {
		return fmt.Errorf("originator %q must be between 1 and 250 bytes, but was: %d", originator, len(originator))
	}

	if !strings.Contains(originator, ".") {
		return nil
	}

	for originatorPart := range strings.SplitSeq(originator, ".") {
		if !is.Between(len(originatorPart), 1, 63) {
			return fmt.Errorf("originator part %q must be between 1 and 63 bytes, but was: %d", originatorPart, len(originatorPart))
		}
	}
	return nil
}

func normalizeOriginator(originator string) string {
	originator = strings.TrimSpace(originator)
	originator = strings.ToLower(originator)
	return originator
}

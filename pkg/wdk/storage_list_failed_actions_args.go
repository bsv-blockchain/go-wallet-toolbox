package wdk

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// ListFailedActionsArgs defines arguments for listing only failed actions
// It reuses pagination and inclusion flags, and adds an Unfail control.
type ListFailedActionsArgs struct {
	Unfail                           *primitives.BooleanDefaultFalse             `json:"unfail,omitempty"`
	Limit                            primitives.PositiveIntegerDefault10Max10000 `json:"limit,omitempty"`
	Offset                           primitives.PositiveInteger                  `json:"offset,omitempty"`
	SeekPermission                   *primitives.BooleanDefaultTrue              `json:"seekPermission,omitempty"`
	IncludeInputs                    *primitives.BooleanDefaultFalse             `json:"includeInputs,omitempty"`
	IncludeOutputs                   *primitives.BooleanDefaultFalse             `json:"includeOutputs,omitempty"`
	IncludeLabels                    *primitives.BooleanDefaultFalse             `json:"includeLabels,omitempty"`
	IncludeInputSourceLockingScripts *primitives.BooleanDefaultFalse             `json:"includeInputSourceLockingScripts,omitempty"`
	IncludeInputUnlockingScripts     *primitives.BooleanDefaultFalse             `json:"includeInputUnlockingScripts,omitempty"`
	IncludeOutputLockingScripts      *primitives.BooleanDefaultFalse             `json:"includeOutputLockingScripts,omitempty"`
	// LabelQueryMode is ignored for failed listing but kept for compatibility
	LabelQueryMode *defs.QueryMode `json:"labelQueryMode,omitempty"`
}

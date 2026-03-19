package validate_test

import (
	"strings"
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestListActionsArgs(t *testing.T) {
	tests := map[string]struct {
		args *wdk.ListActionsArgs
	}{
		"valid labels and defaults": {
			args: &wdk.ListActionsArgs{
				LabelQueryMode: to.Ptr(defs.QueryModeAny),
				Labels:         []primitives.StringUnder300{"valid-label"},
				SeekPermission: to.Ptr(primitives.BooleanDefaultTrue(true)),
			},
		},
		"valid empty string as query mode": {
			args: &wdk.ListActionsArgs{
				LabelQueryMode: to.Ptr(defs.QueryMode("")),
				Labels:         []primitives.StringUnder300{"valid-label"},
				SeekPermission: to.Ptr(primitives.BooleanDefaultTrue(true)),
			},
		},
		"valid args": {
			args: &wdk.ListActionsArgs{
				Limit:          validate.MaxPaginationLimit,
				Offset:         validate.MaxPaginationOffset,
				LabelQueryMode: to.Ptr(defs.QueryModeAll),
				SeekPermission: to.Ptr(primitives.BooleanDefaultTrue(true)),
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			err := validate.ListActionsArgs(test.args)

			// then:
			require.NoError(t, err)
		})
	}
}

func TestWrongListActionsArgs(t *testing.T) {
	tests := map[string]struct {
		args *wdk.ListActionsArgs
	}{
		"nil args": {
			args: nil,
		},
		"limit exceeds max": {
			args: &wdk.ListActionsArgs{Limit: validate.MaxPaginationLimit + 1},
		},
		"offset exceeds max": {
			args: &wdk.ListActionsArgs{Offset: validate.MaxPaginationOffset + 1},
		},
		"invalid labelQueryMode": {
			args: &wdk.ListActionsArgs{LabelQueryMode: to.Ptr(defs.QueryMode("unknown"))},
		},
		"seekPermission set to false": {
			args: &wdk.ListActionsArgs{SeekPermission: to.Ptr(primitives.BooleanDefaultTrue(false))},
		},
		"invalid label - too long": {
			args: &wdk.ListActionsArgs{
				Labels: []primitives.StringUnder300{primitives.StringUnder300(strings.Repeat("x", 301))},
			},
		},
		"invalid label - empty": {
			args: &wdk.ListActionsArgs{
				Labels: []primitives.StringUnder300{""},
			},
		},
		"inconsistent includeInputSourceLockingScripts with no includeInputs": {
			args: &wdk.ListActionsArgs{
				IncludeInputSourceLockingScripts: to.Ptr(primitives.BooleanDefaultFalse(true)),
			},
		},
		"inconsistent includeOutputLockingScripts with no includeOutputs": {
			args: &wdk.ListActionsArgs{
				IncludeOutputLockingScripts: to.Ptr(primitives.BooleanDefaultFalse(true)),
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			args := test.args

			// when:
			err := validate.ListActionsArgs(args)

			// then:
			require.Error(t, err)
		})
	}
}

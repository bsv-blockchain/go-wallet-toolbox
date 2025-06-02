package validate_test

import (
	"strings"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/validate"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func TestListActionsArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    *wdk.ListActionsArgs
		wantErr bool
	}{
		{
			name:    "nil args",
			args:    nil,
			wantErr: true,
		},
		{
			name:    "limit exceeds max",
			args:    &wdk.ListActionsArgs{Limit: validate.MaxPaginationLimit + 1},
			wantErr: true,
		},
		{
			name:    "offset exceeds max",
			args:    &wdk.ListActionsArgs{Offset: validate.MaxPaginationOffset + 1},
			wantErr: true,
		},
		{
			name:    "invalid labelQueryMode",
			args:    &wdk.ListActionsArgs{LabelQueryMode: to.Ptr(primitives.LabelQueryModeString("unknown"))},
			wantErr: true,
		},
		{
			name:    "seekPermissions set to false",
			args:    &wdk.ListActionsArgs{SeekPermissions: to.Ptr(primitives.BooleanDefaultTrue(false))},
			wantErr: true,
		},
		{
			name: "invalid label - too long",
			args: &wdk.ListActionsArgs{
				Labels: []primitives.StringUnder300{primitives.StringUnder300(strings.Repeat("x", 301))},
			},
			wantErr: true,
		},
		{
			name: "valid label and defaults",
			args: &wdk.ListActionsArgs{
				LabelQueryMode:  to.Ptr(primitives.LabelQueryModeString("any")),
				Labels:          []primitives.StringUnder300{"valid-label"},
				SeekPermissions: to.Ptr(primitives.BooleanDefaultTrue(true)),
			},
			wantErr: false,
		},
		{
			name: "valid args",
			args: &wdk.ListActionsArgs{
				Limit:           validate.MaxPaginationLimit,
				Offset:          validate.MaxPaginationOffset,
				LabelQueryMode:  to.Ptr(primitives.LabelQueryModeString("all")),
				SeekPermissions: to.Ptr(primitives.BooleanDefaultTrue(true)),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given:
			args := tt.args

			// When:
			err := validate.ListActionsArgs(args)

			// Then:
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

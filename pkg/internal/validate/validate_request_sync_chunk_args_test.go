package validate_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestValidRequestSyncChunkArgs_Success(t *testing.T) {
	now := time.Now()
	tests := map[string]struct {
		args *wdk.RequestSyncChunkArgs
	}{
		"all valid fields": {
			args: &wdk.RequestSyncChunkArgs{
				FromStorageIdentityKey: "from_key",
				ToStorageIdentityKey:   "to_key",
				IdentityKey:            "identity",
				Since:                  &now,
				MaxRoughSize:           100,
				MaxItems:               10,
				Offsets: []wdk.SyncOffsets{
					{Name: "entity", Offset: 5},
				},
			},
		},
		"minimal valid fields": {
			args: &wdk.RequestSyncChunkArgs{
				FromStorageIdentityKey: "from_key",
				ToStorageIdentityKey:   "to_key",
				IdentityKey:            "identity",
				MaxRoughSize:           1,
				MaxItems:               1,
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validate.ValidRequestSyncChunkArgs(test.args)
			require.NoError(t, err)
		})
	}
}

func TestValidRequestSyncChunkArgs_MissingRequiredFields(t *testing.T) {
	valid := func() *wdk.RequestSyncChunkArgs {
		return &wdk.RequestSyncChunkArgs{
			FromStorageIdentityKey: "from_key",
			ToStorageIdentityKey:   "to_key",
			IdentityKey:            "identity",
			MaxRoughSize:           100,
			MaxItems:               10,
		}
	}
	tests := map[string]struct {
		modify  func(args *wdk.RequestSyncChunkArgs)
		wantErr string
	}{
		"missing toStorageIdentityKey": {
			modify:  func(args *wdk.RequestSyncChunkArgs) { args.ToStorageIdentityKey = "" },
			wantErr: "missing toStorageIdentityKey parameter",
		},
		"missing fromStorageIdentityKey": {
			modify:  func(args *wdk.RequestSyncChunkArgs) { args.FromStorageIdentityKey = "" },
			wantErr: "missing fromStorageIdentityKey parameter",
		},
		"missing user identityKey": {
			modify:  func(args *wdk.RequestSyncChunkArgs) { args.IdentityKey = "" },
			wantErr: "missing user identityKey parameter",
		},
		"maxItems is zero": {
			modify:  func(args *wdk.RequestSyncChunkArgs) { args.MaxItems = 0 },
			wantErr: "maxItems must be greater than 0, got 0",
		},
		"maxRoughSize is zero": {
			modify:  func(args *wdk.RequestSyncChunkArgs) { args.MaxRoughSize = 0 },
			wantErr: "maxRoughSize must be greater than 0, got 0",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			args := valid()
			test.modify(args)
			err := validate.ValidRequestSyncChunkArgs(args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}

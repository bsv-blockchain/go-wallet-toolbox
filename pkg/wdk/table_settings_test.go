package wdk_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestStorageDBTypeOf(t *testing.T) {
	tests := map[string]struct {
		engine   defs.DBType
		expected wdk.StorageDBType
	}{
		"sqlite":            {engine: defs.DBTypeSQLite, expected: wdk.StorageDBTypeSQLite},
		"mysql":             {engine: defs.DBTypeMySQL, expected: wdk.StorageDBTypeMySQL},
		"postgres":          {engine: defs.DBTypePostgres, expected: wdk.StorageDBTypePostgres},
		"mixed case":        {engine: defs.DBType("PostgreS"), expected: wdk.StorageDBTypePostgres},
		"unknown is absent": {engine: defs.DBType("oracle"), expected: ""},
		"empty is absent":   {engine: defs.DBType(""), expected: ""},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, wdk.StorageDBTypeOf(test.engine))
		})
	}
}

func TestTableSettingsJSONDBType(t *testing.T) {
	t.Run("an unknown engine is omitted rather than sent empty", func(t *testing.T) {
		encoded, err := json.Marshal(wdk.TableSettings{StorageIdentityKey: "02aa", Chain: defs.NetworkTestnet})
		require.NoError(t, err)

		var fields map[string]any
		require.NoError(t, json.Unmarshal(encoded, &fields))
		assert.NotContains(t, fields, "dbtype")
	})

	t.Run("a known engine uses the cross-implementation spelling", func(t *testing.T) {
		encoded, err := json.Marshal(wdk.TableSettings{DbType: wdk.StorageDBTypePostgres})
		require.NoError(t, err)

		var fields map[string]any
		require.NoError(t, json.Unmarshal(encoded, &fields))
		assert.Equal(t, "Postgres", fields["dbtype"])
	})
}

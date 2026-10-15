package wdk

import (
	"strings"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// TableSettings is a struct that holds the settings of the whole DB
type TableSettings struct {
	StorageIdentityKey string          `json:"storageIdentityKey"`
	StorageName        string          `json:"storageName"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	Chain              defs.BSVNetwork `json:"chain"`
	DbType             StorageDBType   `json:"dbtype,omitempty"`
	MaxOutputScript    int             `json:"maxOutputScript"`
}

// StorageDBType is the database engine a storage reports in its settings, spelled as the
// cross-implementation wire contract requires (conformance/vectors/wallet/storage). TypeScript
// storage clients reject any other spelling, including an empty string.
type StorageDBType string

// Database engines a storage may report in its settings.
const (
	StorageDBTypeSQLite    StorageDBType = "SQLite"
	StorageDBTypeMySQL     StorageDBType = "MySQL"
	StorageDBTypePostgres  StorageDBType = "Postgres"
	StorageDBTypeIndexedDB StorageDBType = "IndexedDB"
)

// StorageDBTypeOf returns the settings spelling of a configured database engine, or an empty
// StorageDBType, which is omitted from JSON, when the engine has no wire name.
func StorageDBTypeOf(engine defs.DBType) StorageDBType {
	switch defs.DBType(strings.ToLower(string(engine))) {
	case defs.DBTypeSQLite:
		return StorageDBTypeSQLite
	case defs.DBTypeMySQL:
		return StorageDBTypeMySQL
	case defs.DBTypePostgres:
		return StorageDBTypePostgres
	default:
		return ""
	}
}

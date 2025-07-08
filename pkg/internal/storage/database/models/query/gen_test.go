package query

import (
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestQuery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	SetDefault(db)

	Commission.TableName()

	lockingScript := Commission.LockingScript.ColumnName().String()
	t.Log(lockingScript)
}


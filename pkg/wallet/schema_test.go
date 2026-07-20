package wallet

import (
    "testing"
    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "gorm.io/gorm/schema"
)

func TestSchema(t *testing.T) {
    db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "bsv_",
			SingularTable: false,
		},
    })
    db.AutoMigrate(&models.Output{})
    
    var rows []map[string]interface{}
    db.Raw("PRAGMA table_info(bsv_outputs)").Scan(&rows)
    for _, row := range rows {
        t.Logf("COL: %v\n", row["name"])
    }
    t.Fail()
}

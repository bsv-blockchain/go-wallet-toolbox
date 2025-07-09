package main

import (
	"log"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gen"
	"gorm.io/gorm"
)

//go:generate go run gorm_gen.go

func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath: "../genquery",
		Mode:    gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface,
	})

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	g.UseDB(db)

	g.ApplyBasic(models.Commission{})

	// Uncomment and adjust the following lines to generate additional interfaces or methods as needed.
	// For example, to generate a custom interface for CommissionQuerier:
	//g.ApplyInterface(func(CommissionQuerier) {}, models.Commission{})

	g.Execute()
}

// Uncomment the following lines to define a custom interface for querying commissions.
// For example, to get a commission by user ID and transaction ID:
//type CommissionQuerier interface {
//	// SELECT * FROM @@table WHERE user_id = @userID AND transaction_id = @transactionID{{end}}
//	GetByUserIDAndTransactionID(userID int, transactionID uint) (gen.T, error)
//}

package main

import (
	"log"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gen"
	"gorm.io/gorm"
)

//go:generate go run gorm_gen.go

type CommissionQuerier interface {
	// SELECT * FROM @@table WHERE user_id = @userID AND transaction_id = @transactionID{{end}}
	GetByUserIDAndTransactionID(userID int, transactionID uint) (gen.T, error)
}

func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath: "../genquery",
		Mode:    gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface, // generate mode
	})

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	g.UseDB(db)

	// Generate basic type-safe DAO API for struct `model.User` following conventions
	g.ApplyBasic(models.Commission{})

	// Generate Type Safe API with Dynamic SQL defined on Querier interface for `model.User` and `model.Company`
	g.ApplyInterface(func(CommissionQuerier) {}, models.Commission{})

	// Generate the code
	g.Execute()
}

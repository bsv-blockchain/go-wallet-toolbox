package main

import (
	"log"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
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

	g.ApplyBasic(models.Commission{}, models.NumericIDLookup{}, models.OutputBasket{}, models.KnownTx{})

	g.Execute()
}

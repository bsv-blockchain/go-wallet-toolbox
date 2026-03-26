package main

import (
	"log"
	"os"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gen"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
)

//go:generate go run gorm_gen.go

const (
	outPath     = "../genquery"
	genFilePath = outPath + "/gen.go"
)

func main() {
	// Warning: Don't use globally defined variables (like Q) in the generated code.
	// We don't support this approach because there can be multiple instances of storages.
	// Instead, each repository should create its own instance of Query and use it. (genquery.Use(db))

	g := gen.NewGenerator(gen.Config{
		OutPath: outPath,
		Mode:    gen.WithoutContext | gen.WithQueryInterface,
	})

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	g.UseDB(db)

	g.ApplyBasic(
		models.Commission{},
		models.NumericIDLookup{},
		models.OutputBasket{},
		models.KnownTx{},
		models.Transaction{},
		models.Output{},
		models.Label{},
		models.TransactionLabel{},
		models.Tag{},
		models.OutputTag{},
		models.UserUTXO{},
		models.User{},
		models.TxNote{},
		models.Certificate{},
		models.ChaintracksLiveHeader{},
		models.ChaintracksBulkFile{},
	)

	g.Execute()

	// Workaround to substitute generated method that is conflicting with the one in the model "Transaction"
	// NOTE: When you want to create gorm-gen transaction, you should use the method `DBTransaction` instead of `Transaction`.

	log.Println("Applying automated workaround for Transaction method conflict...")
	applyTransactionMethodWorkaround()
}

func applyTransactionMethodWorkaround() {
	const originalMethodSignature = `func (q *Query) Transaction(fc func(tx *Query) error, opts ...*sql.TxOptions) error {`
	const replacementMethodSignature = `func (q *Query) DBTransaction(fc func(tx *Query) error, opts ...*sql.TxOptions) error {`

	input, err := os.ReadFile(genFilePath)
	if err != nil {
		log.Fatalf("WORKAROUND FAILED: Could not read generated file '%s': %v", genFilePath, err)
	}

	fileContent := string(input)

	// Check if the conflicting method exists. If not, the workaround isn't needed,
	// which might happen if gorm/gen updates or your models change.
	if !strings.Contains(fileContent, originalMethodSignature) {
		log.Println("WORKAROUND SKIPPED: Conflicting method signature not found. It may have been fixed or changed.")
		return
	}

	newContent := strings.Replace(fileContent, originalMethodSignature, replacementMethodSignature, 1)

	err = os.WriteFile(genFilePath, []byte(newContent), 0o600) //nolint:gosec // G703 - genFilePath is a hardcoded constant, not user input
	if err != nil {
		log.Fatalf("WORKAROUND FAILED: Could not write changes to generated file %q: %v", genFilePath, err)
	}

	log.Println("WORKAROUND SUCCESS: Renamed conflicting Transaction method to DBTransaction.")
	log.Println(`NOTE: When you want to create gorm-gen transaction, you need to use the method "DBTransaction" instead of "Transaction".`)
}

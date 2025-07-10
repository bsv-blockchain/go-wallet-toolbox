package main

import (
	"log"
	"os"
	"strings"

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

	g.ApplyBasic(
		models.Commission{},
		models.NumericIDLookup{},
		models.OutputBasket{},
		models.KnownTx{},
		models.Transaction{},
		models.Output{},
		models.Label{},
		models.TransactionLabel{},
	)

	g.Execute()

	// Workaround to substitute generated method that is conflicting with the one in the model "Transaction"

	log.Println("Applying automated workaround for Transaction method conflict...")
	applyTransactionMethodWorkaround("../genquery/gen.go")
}

func applyTransactionMethodWorkaround(filePath string) {
	// Read the entire content of the generated file.
	input, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		log.Fatalf("WORKAROUND FAILED: Could not read generated file '%s': %v", filePath, err)
	}

	fileContent := string(input)

	// Define the exact method signature that gorm/gen creates, which causes the conflict.
	originalMethodSignature := `func (q *Query) Transaction(fc func(tx *Query) error, opts ...*sql.TxOptions) error {`

	// Define the new, non-conflicting method signature.
	replacementMethodSignature := `func (q *Query) DBTransaction(fc func(tx *Query) error, opts ...*sql.TxOptions) error {`

	// Check if the conflicting method exists. If not, the workaround isn't needed,
	// which might happen if gorm/gen updates or your models change.
	if !strings.Contains(fileContent, originalMethodSignature) {
		log.Println("WORKAROUND SKIPPED: Conflicting method signature not found. It may have been fixed or changed.")
		return
	}

	// Replace the first occurrence of the conflicting method signature with the new one.
	newContent := strings.Replace(fileContent, originalMethodSignature, replacementMethodSignature, 1)

	// Write the modified content back to the file, overwriting the original.
	err = os.WriteFile(filePath, []byte(newContent), 0600)
	if err != nil {
		log.Fatalf("WORKAROUND FAILED: Could not write changes to generated file %q: %v", filePath, err)
	}

	log.Println("WORKAROUND SUCCESS: Renamed conflicting Transaction method to DBTransaction.")
}

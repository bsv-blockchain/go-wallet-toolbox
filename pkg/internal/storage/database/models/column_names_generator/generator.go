//go:build gen
package main

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/tools/gorm-names-gen"
)

func main() {
	modelsToGenerate := map[string]interface{}{
		"Setting": &models.Setting{},
		"User": &models.User{},
		"OutputBasket": &models.OutputBasket{},
		"CertificateField": &models.CertificateField{},
		"Certificate": &models.Certificate{},
		"UserUTXO": &models.UserUTXO{},
		"Transaction": &models.Transaction{},
		"Output": &models.Output{},
		"KnownTx": &models.KnownTx{},
		"Label": &models.Label{},
		"TransactionLabels": &models.TransactionLabels{},
		"NumericIDLookup": &models.NumericIDLookup{},
		"SyncState": &models.SyncState{},
		"KeyValue": &models.KeyValue{},
		"Tag": &models.Tag{},
		"OutputTags": &models.OutputTags{},
		"Commission": &models.Commission{},
	}

	modelgen.Generate(modelsToGenerate, ".", "column_names_generated.go", "models")
}

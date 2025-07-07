package wallet

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func (w *Wallet) assemblyTransaction(createActionResult *wdk.StorageCreateActionResult, args sdk.CreateActionArgs) (*transaction.Transaction, error) {
	txAssembler := mapping.NewCreateActionTransactionAssembler(w.keyDeriver, createActionResult, args)
	tx, err := txAssembler.Assemble()
	if err != nil {
		return nil, fmt.Errorf("failed to assemble transaction from storage response: %w", err)
	}
	return tx, nil
}

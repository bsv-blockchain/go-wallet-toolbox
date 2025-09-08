package methods

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
)

type FaucetOutput struct {
	Address string `json:"address"`
	Amount  uint64 `json:"amount"`
}

// FundAddress creates and broadcasts a faucet payment with one or more outputs.
// Returns the txid string and full Atomic BEEF hex on success.
func FundAddress(ctx context.Context, deps FaucetDeps, outputs ...FaucetOutput) (string, string, error) {
	if len(outputs) == 0 {
		return "", "", fmt.Errorf("at least one output is required")
	}

	if deps.Storage == nil {
		return "", "", fmt.Errorf("storage provider not configured")
	}

	// Create outputs for each address and amount
	createOutputs := make([]sdk.CreateActionOutput, len(outputs))
	for i, output := range outputs {
		addr, err := script.NewAddressFromString(output.Address)
		if err != nil {
			return "", "", fmt.Errorf("invalid address[%d] %s: %w", i, output.Address, err)
		}
		lockingScript, err := p2pkh.Lock(addr)
		if err != nil {
			return "", "", fmt.Errorf("p2pkh lock for address[%d] %s: %w", i, output.Address, err)
		}

		createOutputs[i] = sdk.CreateActionOutput{
			LockingScript:     lockingScript.Bytes(),
			Satoshis:          output.Amount,
			OutputDescription: fmt.Sprintf("Faucet funding to %s", output.Address),
			Tags:              []string{"faucet"},
		}
	}

	createArgs := sdk.CreateActionArgs{
		Description: "Faucet payment with multiple outputs",
		Outputs:     createOutputs,
		Labels:      []string{"faucet_funding"},
		Options: &sdk.CreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr(false),
			RandomizeOutputs:       to.Ptr(false),
		},
	}

	result, err := deps.Wallet.CreateAction(ctx, createArgs, "")
	if err != nil {
		return "", "", fmt.Errorf("create action: %w", err)
	}

	beefHex := ""
	if len(result.Tx) > 0 {
		beefHex = hex.EncodeToString(result.Tx)
	}

	return result.Txid.String(), beefHex, nil
}

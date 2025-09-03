package methods

import (
	"context"
	"encoding/hex"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

type FaucetDeps struct {
	FaucetKeyHex         string
	Network              defs.BSVNetwork
	Storage              wdk.WalletStorageProvider
	MaxFaucetTotalAmount uint64 // 0 means unlimited
	Wallet               sdk.Interface
}

type FaucetOutput struct {
	Address string `json:"address"`
	Amount  uint64 `json:"amount"`
}

// FundAddress creates and broadcasts a faucet payment with one or more outputs.
// Returns the txid string and full Atomic BEEF hex on success.
func FundAddress(ctx context.Context, deps FaucetDeps, outputs ...FaucetOutput) (string, string, error) {
	if deps.FaucetKeyHex == "" {
		return "", "", fmt.Errorf("faucet key not configured")
	}
	if len(outputs) == 0 {
		return "", "", fmt.Errorf("at least one output is required")
	}

	if deps.Storage == nil {
		return "", "", fmt.Errorf("storage provider not configured")
	}

	faucetPriv, err := ec.PrivateKeyFromHex(deps.FaucetKeyHex)
	if err != nil {
		return "", "", fmt.Errorf("invalid faucet key: %w", err)
	}

	faucetWallet := deps.Wallet
	if faucetWallet == nil {
		faucetWallet, err = wallet.New(deps.Network, faucetPriv, deps.Storage)
		if err != nil {
			return "", "", fmt.Errorf("create wallet: %w", err)
		}
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
		},
	}

	result, err := faucetWallet.CreateAction(ctx, createArgs, "")
	if err != nil {
		return "", "", fmt.Errorf("create action: %w", err)
	}

	beefHex := ""
	if len(result.Tx) > 0 {
		beefHex = hex.EncodeToString(result.Tx)
	}

	return result.Txid.String(), beefHex, nil
}

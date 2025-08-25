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
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/go-softwarelab/common/pkg/to"
)

type FaucetDeps struct {
	FaucetKeyHex string
	Network      defs.BSVNetwork
	ServerURL    string
}

// FundAddress creates and broadcasts a faucet payment to the faucet's own BRC-29 address.
// Returns the txid string and full Atomic BEEF hex on success.
func FundAddress(ctx context.Context, deps FaucetDeps, address string, amount uint64) (string, string, error) {
	if deps.FaucetKeyHex == "" {
		return "", "", fmt.Errorf("faucet key not configured")
	}
	if amount == 0 {
		return "", "", fmt.Errorf("invalid amount")
	}

	storageClient, cleanup, err := storage.NewClient(deps.ServerURL)
	if err != nil {
		return "", "", fmt.Errorf("connect storage: %w", err)
	}
	defer cleanup()

	faucetPriv, err := ec.PrivateKeyFromHex(deps.FaucetKeyHex)
	if err != nil {
		return "", "", fmt.Errorf("invalid faucet key: %w", err)
	}

	faucetWallet, err := wallet.New(deps.Network, faucetPriv, storageClient)
	if err != nil {
		return "", "", fmt.Errorf("create wallet: %w", err)
	}

	faucetAddr, err := DeriveAddress(deps.FaucetKeyHex, deps.Network)
	if err != nil {
		return "", "", fmt.Errorf("failed to derive faucet address: %w", err)
	}

	addr, err := script.NewAddressFromString(faucetAddr)
	if err != nil {
		return "", "", fmt.Errorf("invalid faucet address: %w", err)
	}
	lockingScript, err := p2pkh.Lock(addr)
	if err != nil {
		return "", "", fmt.Errorf("p2pkh lock: %w", err)
	}

	createArgs := sdk.CreateActionArgs{
		Description: "Faucet payment",
		Outputs: []sdk.CreateActionOutput{{
			LockingScript:     lockingScript.Bytes(),
			Satoshis:          amount,
			OutputDescription: "Faucet funding",
			Tags:              []string{"faucet"},
		}},
		Labels: []string{"faucet_funding"},
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

package internal

import (
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

func (m *Manager) CreateActionWithP2pkh(user fixtures.UserConfig, recipientAddress string, satoshis uint64) (string, fixtures.Summary, error) {
	var summary fixtures.Summary

	summary = append(summary, fmt.Sprintf("Using wallet for user %s", user.Name))

	userWallet, err := m.WalletForUser(user)
	if err != nil {
		return "", summary, fmt.Errorf("failed to get wallet for user %s: %w", user.Name, err)
	}

	summary = append(summary, fmt.Sprintf("Creating P2PKH output to %s for %d satoshis", recipientAddress, satoshis))

	addr, err := script.NewAddressFromString(recipientAddress)
	if err != nil {
		return "", summary, fmt.Errorf("failed to parse address %q: %w", recipientAddress, err)
	}

	lockingScript, err := p2pkh.Lock(addr)
	if err != nil {
		return "", summary, fmt.Errorf("failed to create P2PKH locking script: %w", err)
	}

	createArgs := sdk.CreateActionArgs{
		Description: fmt.Sprintf("Create P2PKH action for user %s at %s", user.Name, time.Now().Format(time.RFC3339)),
		Outputs: []sdk.CreateActionOutput{
			{
				LockingScript:     lockingScript.Bytes(),
				Satoshis:          satoshis,
				OutputDescription: "Payment to recipient",
				Tags:              []string{"payment"},
			},
		},
		Labels: []string{"create action with p2pkh", user.Name},
		Options: &sdk.CreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr(false),
		},
	}

	summary = append(summary, fmt.Sprintf("Create action args: %#v", createArgs))

	result, err := userWallet.CreateAction(m.ctx, createArgs, "")
	if err != nil {
		return "", summary, fmt.Errorf("failed to create action for user %s: %w", user.Name, err)
	}

	summary = append(summary, fmt.Sprintf("Create action result: %#v", result))

	txID := result.Txid.String()
	summary = append(summary, fmt.Sprintf("TxID: %s", result.Txid.String()))

	return txID, summary, nil
}

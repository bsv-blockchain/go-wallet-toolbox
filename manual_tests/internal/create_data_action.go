package internal

import (
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

func (m *Manager) CreateActionWithData(user fixtures.UserConfig, data string) (string, fixtures.Summary, error) {
	var summary fixtures.Summary

	summary = append(summary, fmt.Sprintf("Using wallet for user %s", user.Name))

	userWallet, err := m.WalletForUser(user)
	if err != nil {
		return "", summary, fmt.Errorf("failed to get wallet for user %s: %w", user.Name, err)
	}

	summary = append(summary, fmt.Sprintf("Creating data output with data: %s", data))

	dataOutput, err := transaction.CreateOpReturnOutput([][]byte{[]byte(data)})
	if err != nil {
		return "", summary, fmt.Errorf("failed to create OP_RETURN output with data %q: %w", data, err)
	}

	createArgs := sdk.CreateActionArgs{
		Description: fmt.Sprintf("Create action for user %s with data at %s", user.Name, time.Now().Format(time.RFC3339)),
		Outputs: []sdk.CreateActionOutput{
			{
				LockingScript:     dataOutput.LockingScript.Bytes(),
				Satoshis:          0,
				OutputDescription: "Data output",
				Tags:              []string{"data"},
			},
		},
		Labels: []string{"create action with data", user.Name},
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

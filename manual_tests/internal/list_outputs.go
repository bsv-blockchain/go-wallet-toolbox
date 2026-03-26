package internal

import (
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

// ListOutputs lists outputs with pagination and returns a textual summary for display in TUI.
func (m *Manager) ListOutputs(user fixtures.UserConfig, limit, offset uint32, includeLabels bool, basket string) (fixtures.Summary, error) {
	var summary fixtures.Summary

	summary = append(summary, fmt.Sprintf("Using wallet for user %s", user.Name))

	userWallet, err := m.WalletForUser(user)
	if err != nil {
		return summary, fmt.Errorf("failed to get wallet for user %s: %w", user.Name, err)
	}

	args := sdk.ListOutputsArgs{
		Basket:        basket,
		Limit:         &limit,
		Offset:        &offset,
		IncludeLabels: &includeLabels,
	}

	summary = append(summary, fmt.Sprintf("ListOutputsArgs: %#v", args))

	outputs, err := userWallet.ListOutputs(m.ctx, args, "")
	if err != nil {
		return summary, fmt.Errorf("failed to list outputs: %w", err)
	}

	summary = append(summary, fmt.Sprintf("Returned %d outputs (next offset %d)", len(outputs.Outputs), int(offset)+len(outputs.Outputs)))

	for i, out := range outputs.Outputs {
		outpoint := fmt.Sprintf("%s:%d ", out.Outpoint.Txid.String(), out.Outpoint.Index)
		summary = append(summary, fmt.Sprintf("%d) %ssatoshis=%d", i+1, outpoint, out.Satoshis))
	}

	return summary, nil
}

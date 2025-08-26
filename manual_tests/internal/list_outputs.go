package internal

import (
	"fmt"
	"strconv"
	"strings"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

// ListOutputs lists outputs with pagination and returns a textual summary for display in TUI.
func (m *Manager) ListOutputs(user fixtures.UserConfig, limit uint32, offset uint32, includeLabels bool, basket string) (fixtures.Summary, error) {
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

	if len(outputs.Outputs) == 0 {
		summary = append(summary, "No outputs found.")
		return summary, nil
	}

	summary = append(summary, "")
	summary = append(summary, "📋 WALLET OUTPUTS:")
	summary = append(summary, strings.Repeat("─", 80))

	for i, out := range outputs.Outputs {
		// Format transaction ID (show first 8 + last 8 chars with ellipsis)
		txid := out.Outpoint.Txid.String()
		var formattedTxid string
		if len(txid) > 20 {
			formattedTxid = fmt.Sprintf("%s...%s", txid[:8], txid[len(txid)-8:])
		} else {
			formattedTxid = txid
		}

		// Format satoshi amount with commas
		satoshiStr := formatNumberWithCommas(int64(out.Satoshis))
		
		// Create status indicator
		status := "✅ Spendable"
		if !out.Spendable {
			status = "❌ Not Spendable"
		}

		// Main output info
		summary = append(summary, fmt.Sprintf("💰 Output #%d", i+1))
		summary = append(summary, fmt.Sprintf("   🔗 Outpoint: %s:%d", formattedTxid, out.Outpoint.Index))
		summary = append(summary, fmt.Sprintf("   💎 Amount:   %s satoshis", satoshiStr))
		summary = append(summary, fmt.Sprintf("   📊 Status:   %s", status))

		// Add optional fields if present
		if len(out.Labels) > 0 {
			summary = append(summary, fmt.Sprintf("   🏷️  Labels:   %s", strings.Join(out.Labels, ", ")))
		}
		
		if len(out.Tags) > 0 {
			summary = append(summary, fmt.Sprintf("   🔖 Tags:     %s", strings.Join(out.Tags, ", ")))
		}

		if out.CustomInstructions != "" {
			summary = append(summary, fmt.Sprintf("   📝 Custom:   %s", out.CustomInstructions))
		}

		// Add separator between outputs (except for the last one)
		if i < len(outputs.Outputs)-1 {
			summary = append(summary, "")
		}
	}

	return summary, nil
}

// formatNumberWithCommas adds comma separators to large numbers for better readability
func formatNumberWithCommas(n int64) string {
	str := strconv.FormatInt(n, 10)
	if len(str) <= 3 {
		return str
	}

	var result []string
	for i, char := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ",")
		}
		result = append(result, string(char))
	}
	return strings.Join(result, "")
}

package internal

import (
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func (m *Manager) Balance(user fixtures.UserConfig) (uint64, error) {
	userWallet, err := m.WalletForUser(user)
	if err != nil {
		return 0, fmt.Errorf("failed to get wallet for user %s: %w", user.Name, err)
	}

	var balance uint64
	var offset uint32
	limit := uint32(1000)

	for {

		args := sdk.ListOutputsArgs{
			Basket: wdk.BasketNameForChange,
			Limit:  &limit,
			Offset: &offset,
		}

		outputs, err := userWallet.ListOutputs(m.ctx, args, "")
		if err != nil {
			return 0, fmt.Errorf("failed to list outputs for user %s: %w", user.Name, err)
		}

		// Sum the satoshis from all outputs in this page
		for _, output := range outputs.Outputs {
			balance += output.Satoshis
		}

		// Update offset for next page
		offset += uint32(len(outputs.Outputs)) //nolint:gosec // safe: output count fits in uint32

		// Break if we've retrieved all outputs
		if len(outputs.Outputs) < int(limit) {
			break
		}
	}

	return balance, nil
}

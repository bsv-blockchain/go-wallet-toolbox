package internal

import (
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

func (m *Manager) ActionsStats(user fixtures.UserConfig) (map[string]int, error) {
	userWallet, err := m.WalletForUser(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet for user %s: %w", user.Name, err)
	}

	stats := make(map[string]int)
	var offset uint32
	limit := uint32(1000)

	for {

		args := sdk.ListActionsArgs{
			Limit:  &limit,
			Offset: &offset,
		}

		actions, err := userWallet.ListActions(m.ctx, args, "")
		if err != nil {
			return nil, fmt.Errorf("failed to list actions for user %s: %w", user.Name, err)
		}

		for _, action := range actions.Actions {
			strStatus := string(action.Status)
			stats[strStatus] = stats[strStatus] + 1
		}

		if len(actions.Actions) < int(limit) {
			break
		}
	}

	return stats, nil
}

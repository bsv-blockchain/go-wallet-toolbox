package internal

import (
	"fmt"
	"slices"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/tui"
)

type noSendTransactionsResult struct {
	TxHashes      []chainhash.Hash
	NoSendTimes   []time.Duration
	NoSendChange  []transaction.Outpoint
	TotalTxCount  int
	MinNoSendTime time.Duration
	MaxNoSendTime time.Duration
	AvgNoSendTime time.Duration
}

type sendWithTransactionResult struct {
	SendWithTime     time.Duration
	Success          bool
	TxHash           *chainhash.Hash
	BroadcastedTxIds []chainhash.Hash
}

func (m *Manager) ExecuteNoSendSendWith(user fixtures.UserConfig, txCount int, dataPrefix string) (*tui.NoSendSendWithResult, error) {
	noSendResult, err := m.executeNoSendTransactions(user, txCount, dataPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to execute noSend transactions: %w", err)
	}

	sendWithResult, err := m.executeSendWithTransaction(user, noSendResult.TxHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to execute sendWith transaction: %w", err)
	}

	return &tui.NoSendSendWithResult{
		NoSendTimes:      noSendResult.NoSendTimes,
		SendWithTime:     sendWithResult.SendWithTime,
		MinNoSendTime:    noSendResult.MinNoSendTime,
		MaxNoSendTime:    noSendResult.MaxNoSendTime,
		AvgNoSendTime:    noSendResult.AvgNoSendTime,
		TotalTxCount:     noSendResult.TotalTxCount,
		BroadcastedTxIds: sendWithResult.BroadcastedTxIds,
	}, nil
}

func (m *Manager) executeNoSendTransactions(user fixtures.UserConfig, txCount int, dataPrefix string) (*noSendTransactionsResult, error) {
	userWallet, err := m.WalletForUser(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet for user %s: %w", user.Name, err)
	}

	var noSendTimes []time.Duration
	var txHashes []chainhash.Hash
	var noSendChange []transaction.Outpoint

	for i := range txCount {
		startTime := time.Now()

		opReturnData := fmt.Sprintf("%s_%d", dataPrefix, i+1)

		dataOutput, err := transaction.CreateOpReturnOutput([][]byte{[]byte(opReturnData)})
		if err != nil {
			return nil, fmt.Errorf("failed to create OP_RETURN output: %w", err)
		}

		createArgs := sdk.CreateActionArgs{
			Outputs: []sdk.CreateActionOutput{
				{
					LockingScript:     dataOutput.LockingScript.Bytes(),
					Satoshis:          0,
					OutputDescription: fmt.Sprintf("OP_RETURN data output %d", i+1),
				},
			},
			Description: fmt.Sprintf("NoSend transaction %d/%d", i+1, txCount),
			Options: &sdk.CreateActionOptions{
				NoSend:       to.Ptr(true),
				NoSendChange: noSendChange,
			},
		}

		result, err := userWallet.CreateAction(m.ctx, createArgs, "nosend_test")
		if err != nil {
			return nil, fmt.Errorf("failed to create noSend transaction %d: %w", i+1, err)
		}

		duration := time.Since(startTime)
		noSendTimes = append(noSendTimes, duration)

		if result.Txid.Size() != 0 {
			txHashes = append(txHashes, result.Txid)
		}

		if result.NoSendChange != nil {
			noSendChange = result.NoSendChange
		}
	}

	result := &noSendTransactionsResult{
		TxHashes:     txHashes,
		NoSendTimes:  noSendTimes,
		NoSendChange: noSendChange,
		TotalTxCount: txCount,
	}

	if len(noSendTimes) > 0 {
		var total time.Duration
		for _, t := range noSendTimes {
			total += t
		}

		result.MinNoSendTime = slices.Min(noSendTimes)
		result.MaxNoSendTime = slices.Max(noSendTimes)
		result.AvgNoSendTime = total / time.Duration(len(noSendTimes))
	}

	return result, nil
}

func (m *Manager) executeSendWithTransaction(user fixtures.UserConfig, txHashes []chainhash.Hash) (*sendWithTransactionResult, error) {
	userWallet, err := m.WalletForUser(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet for user %s: %w", user.Name, err)
	}

	startTime := time.Now()

	sendWithArgs := sdk.CreateActionArgs{
		Description: "SendWith broadcast transaction",
		Options: &sdk.CreateActionOptions{
			SendWith: txHashes,
		},
	}

	result, err := userWallet.CreateAction(m.ctx, sendWithArgs, "sendwith_test")
	if err != nil {
		return &sendWithTransactionResult{
			SendWithTime:     time.Since(startTime),
			Success:          false,
			TxHash:           nil,
			BroadcastedTxIds: txHashes,
		}, fmt.Errorf("failed to execute sendWith: %w", err)
	}

	duration := time.Since(startTime)

	var txHash *chainhash.Hash
	if result.Txid.Size() != 0 {
		txHash = &result.Txid
	}

	return &sendWithTransactionResult{
		SendWithTime:     duration,
		Success:          true,
		TxHash:           txHash,
		BroadcastedTxIds: txHashes,
	}, nil
}

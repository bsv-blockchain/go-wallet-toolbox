package internal

import (
	"context"
	"fmt"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"log/slog"
	"time"
)

type Manager struct {
	ctx    context.Context
	config *fixtures.Config

	storageInfra *StorageInfra
}

func NewManager(ctx context.Context, config *fixtures.Config) *Manager {
	return &Manager{
		ctx:    ctx,
		config: config,
	}
}

func (m *Manager) Ctx() context.Context {
	return m.ctx
}

func (m *Manager) SelectStorageType(storageType fixtures.StorageType) error {
	switch storageType {
	case fixtures.StorageTypeLocalSQLite:
		storage, err := CreateLocalStorage(m.ctx, m.config.BSVNetwork, m.config.ServerPrivateKey)
		if err != nil {
			return fmt.Errorf("failed to create local storage: %w", err)
		}

		m.storageInfra = storage
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}

	return nil
}

func (m *Manager) WalletForUser(user fixtures.UserConfig) (sdk.Interface, error) {
	userWallet, err := wallet.New(m.config.BSVNetwork, user.PrivKey, m.storageInfra.Provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet for user %s: %w", user.Name, err)
	}

	return userWallet, nil
}

func (m *Manager) Panic(err error, msg string) {
	slog.Default().Error(msg, err)
	// TODO Close storage if needed
}

func (m *Manager) GetWalletConfigs() []fixtures.UserConfig {
	return []fixtures.UserConfig{
		m.config.Alice,
		m.config.Bob,
	}
}

func (m *Manager) GetBSVNetwork() defs.BSVNetwork {
	return m.config.BSVNetwork
}

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
		offset += uint32(len(outputs.Outputs))

		// Break if we've retrieved all outputs
		if len(outputs.Outputs) < int(limit) {
			break
		}
	}

	return balance, nil
}

func (m *Manager) InternalizeTxID(txID string, user fixtures.UserConfig, keyID brc29.KeyID, address string) (fixtures.Summary, error) {
	var summary fixtures.Summary

	txIDHash, err := chainhash.NewHashFromHex(txID)
	if err != nil {
		return summary, fmt.Errorf("invalid txID %q: %w", txID, err)
	}

	summary = append(summary, fmt.Sprintf("Fetching atomic beef for txID %q on network %q", txID, m.GetBSVNetwork()))

	beef, err := m.storageInfra.Services.GetBEEF(m.ctx, txID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get beef for txID %q, network %q: %w", txID, m.GetBSVNetwork(), err)
	}

	atomicBeef, err := beef.AtomicBytes(txIDHash)
	if err != nil {
		return summary, fmt.Errorf("failed to get atomic bytes for txID %q: %w", txID, err)
	}

	summary = append(summary, fmt.Sprintf("Fetched atomic beef for txID %q: %x", txID, atomicBeef))

	tx, err := transaction.NewTransactionFromBEEF(atomicBeef)
	if err != nil {
		return summary, fmt.Errorf("failed to create transaction from atomic beef: %w", err)
	}

	addressObj, err := script.NewAddressFromString(address)
	if err != nil {
		return summary, fmt.Errorf("failed to parse address %q: %w", address, err)
	}

	lockingScript, err := p2pkh.Lock(addressObj)
	if err != nil {
		return summary, fmt.Errorf("failed to create locking script for address %q: %w", address, err)
	}

	summary = append(summary, fmt.Sprintf("Created locking script for address %q: %q", address, lockingScript.String()))

	var vouts []int
	for vout, output := range tx.Outputs {
		if output.LockingScript.Equals(lockingScript) {
			vouts = append(vouts, vout)
		}
	}

	if len(vouts) == 0 {
		return summary, fmt.Errorf("no outputs found for address %q in transaction %q", address, txID)
	}

	summary = append(summary, fmt.Sprintf("Found %d outputs for address %q in transaction %q", len(vouts), address, txID))

	derivationPrefixBytes, err := BytesFromBase64(keyID.DerivationPrefix)
	if err != nil {
		return summary, fmt.Errorf("failed to convert derivation prefix from base64: %w", err)
	}

	derivationSuffixBytes, err := BytesFromBase64(keyID.DerivationSuffix)
	if err != nil {
		return summary, fmt.Errorf("failed to convert derivation suffix from base64: %w", err)
	}

	summary = append(summary, "Converted derivation prefix and suffix to bytes (Using base64.StdEncoding.DecodeString!)")

	userWallet, err := m.WalletForUser(user)
	if err != nil {
		return summary, fmt.Errorf("failed to get wallet for user %s: %w", user.Name, err)
	}

	_, anyonePub := sdk.AnyoneKey()
	internalizeArgs := sdk.InternalizeActionArgs{
		Tx: atomicBeef,
		Outputs: slices.Map(vouts, func(vout int) sdk.InternalizeOutput {
			return sdk.InternalizeOutput{
				OutputIndex: uint32(vout),
				Protocol:    "wallet payment",
				PaymentRemittance: &sdk.Payment{
					DerivationPrefix:  derivationPrefixBytes,
					DerivationSuffix:  derivationSuffixBytes,
					SenderIdentityKey: anyonePub,
				},
			}
		}),
		Labels:      []string{"internalize", txID},
		Description: fmt.Sprintf("internalize from faucet: %s", time.Now().Format(time.RFC3339)),
	}

	summary = append(summary, fmt.Sprintf("Generated internalize args with labels %#v", internalizeArgs.Labels))

	internalizeResult, err := userWallet.InternalizeAction(m.ctx, internalizeArgs, "")
	if err != nil {
		return summary, fmt.Errorf("failed to internalize action for user %s: %w", user.Name, err)
	}

	summary = append(summary, fmt.Sprintf("Internalized action for user %s: %#v", user.Name, internalizeResult))

	return summary, nil
}

func (m *Manager) CreateActionWithData(user fixtures.UserConfig, data string) (fixtures.Summary, error) {
	var summary fixtures.Summary

	summary = append(summary, fmt.Sprintf("Using wallet for user %s", user.Name))

	userWallet, err := m.WalletForUser(user)
	if err != nil {
		return summary, fmt.Errorf("failed to get wallet for user %s: %w", user.Name, err)
	}

	summary = append(summary, fmt.Sprintf("Creating data output with data: %s", data))

	dataOutput, err := transaction.CreateOpReturnOutput([][]byte{[]byte(data)})
	if err != nil {
		return summary, fmt.Errorf("failed to create OP_RETURN output with data %q: %w", data, err)
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
		return summary, fmt.Errorf("failed to create action for user %s: %w", user.Name, err)
	}

	summary = append(summary, fmt.Sprintf("Create action result: %#v", result))

	summary = append(summary, fmt.Sprintf("TxID: %s", result.Txid.String()))

	return summary, nil
}

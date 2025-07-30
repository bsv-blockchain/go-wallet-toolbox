package main

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

var (
	// DefaultRecipientAddress is the address to send satoshis to (P2PKH address)
	DefaultRecipientAddress = "1A6ut1tWnfg5mAD8s1drDLM6gNsLNGvgWq"

	// DefaultSatoshisToSend is the amount to send to the recipient
	DefaultSatoshisToSend = uint64(100)

	// DefaultOutputDescription describes the purpose of this output
	DefaultOutputDescription = "Payment to recipient"

	// DefaultTransactionDescription describes the purpose of this transaction
	DefaultTransactionDescription = "Create action example transaction"

	// DefaultOriginator specifies the originator domain or FQDN used to identify the source of the action request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	DefaultOriginator = "example.com"

)

// createLockingScript creates a P2PKH locking script from the recipient address
func createLockingScript(address string) ([]byte, error) {
	addr, err := script.NewAddressFromString(address)
	if err != nil {
		return nil, fmt.Errorf("failed to parse address: %w", err)
	}

	lockingScript, err := p2pkh.Lock(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create P2PKH script: %w", err)
	}

	return lockingScript.Bytes(), nil
}

// createActionArgs creates the arguments needed for the CreateAction
func createActionArgs() (sdk.CreateActionArgs, error) {
	lockingScript, err := createLockingScript(DefaultRecipientAddress)
	if err != nil {
		return sdk.CreateActionArgs{}, fmt.Errorf("failed to create locking script: %w", err)
	}

	signAndProcess := true
	return sdk.CreateActionArgs{
		Description: DefaultTransactionDescription,
		Outputs: []sdk.CreateActionOutput{
			{
				LockingScript:     lockingScript,
				Satoshis:          DefaultSatoshisToSend,
				OutputDescription: DefaultOutputDescription,
				Tags:              []string{"payment", "example"},
			},
		},
		Labels: []string{"create_action_example"},
		Options: &sdk.CreateActionOptions{
			SignAndProcess: &signAndProcess,
		},
	}, nil
}

// This example demonstrates how to create and send a Bitcoin transaction using Alice's wallet.
// The wallet automatically selects UTXOs, creates change outputs, calculates fees, and broadcasts the transaction.
func main() {
	show.ProcessStart("Create Action")
	ctx := context.Background()

	show.Step("Alice", "Creating wallet and setting up environment")
	alice := example_setup.CreateAlice()

	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	show.Info("Recipient address", DefaultRecipientAddress)

	createArgs, err := createActionArgs()
	if err != nil {
		panic(err)
	}

	show.Step("Alice", fmt.Sprintf("Creating transaction to send %d satoshis", DefaultSatoshisToSend))
	show.Info("Transaction description", DefaultTransactionDescription)
	show.Info("Output description", DefaultOutputDescription)

	result, err := aliceWallet.CreateAction(ctx, createArgs, DefaultOriginator)
	if err != nil {
		panic(fmt.Errorf("failed to create action: %w", err))
	}

	show.WalletSuccess("CreateAction", createArgs, *result)

	if result.Txid.String() != "" {
		show.Transaction(result.Txid.String())
		show.Info("Status", "Transaction successfully created and broadcast")

		if len(result.SendWithResults) > 0 {
			show.Info("Broadcast status", result.SendWithResults[0].Status)
		}
	}

	show.Success("Transaction created and sent successfully")
	show.ProcessComplete("Create Action")
}

package main

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

var (
	// RecipientAddress is the address to send satoshis to (P2PKH address)
	RecipientAddress = "" // example: 1A6ut1tWnfg5mAD8s1drDLM6gNsLNGvgWq

	// SatoshisToSend is the amount to send to the recipient
	SatoshisToSend = uint64(1) // example: 100

	// OutputDescription describes the purpose of this output
	OutputDescription = "Payment to recipient"

	// TransactionDescription describes the purpose of this transaction
	TransactionDescription = "Create P2PKH Transaction Example"

	// Originator specifies the originator domain or FQDN used to identify the source of the action request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	Originator = "example.com"
)

// This example demonstrates how to create and send a Bitcoin transaction using Alice's wallet.
// The wallet automatically selects UTXOs, creates change outputs, calculates fees, and broadcasts the transaction.
func main() {
	show.ProcessStart("Create P2PKH Transaction")
	ctx := context.Background()

	if RecipientAddress == "" {
		panic(fmt.Errorf("recipient address must be provided"))
	}

	if SatoshisToSend == 0 {
		panic(fmt.Errorf("satoshis to send must be provided"))
	}

	show.Step("Alice", "Creating wallet and setting up environment")
	alice := example_setup.CreateAlice()

	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	show.Info("Recipient address", RecipientAddress)

	// Create P2PKH locking script from the recipient address
	addr, err := script.NewAddressFromString(RecipientAddress)
	if err != nil {
		panic(fmt.Errorf("failed to parse address: %w", err))
	}

	lockingScript, err := p2pkh.Lock(addr)
	if err != nil {
		panic(fmt.Errorf("failed to create P2PKH script: %w", err))
	}

	// Create the arguments needed for the CreateAction
	createArgs := sdk.CreateActionArgs{
		Description: TransactionDescription,
		Outputs: []sdk.CreateActionOutput{
			{
				LockingScript:     lockingScript.Bytes(),
				Satoshis:          SatoshisToSend,
				OutputDescription: OutputDescription,
				Tags:              []string{"payment", "example"},
			},
		},
		Labels: []string{"create_action_example"},
		Options: &sdk.CreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr(false),
		},
	}

	show.Step("Alice", fmt.Sprintf("Creating transaction to send %d satoshis", SatoshisToSend))
	show.Info("Transaction description", TransactionDescription)
	show.Info("Output description", OutputDescription)

	result, err := aliceWallet.CreateAction(ctx, createArgs, Originator)
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
	show.ProcessComplete("Create P2PKH Transaction")
}

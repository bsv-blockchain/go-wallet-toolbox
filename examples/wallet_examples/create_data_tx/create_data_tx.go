package main

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

const (
	// DataToEmbed is the string that will be embedded in an OP_RETURN output
	// example: "hello world"
	DataToEmbed = "hello world"

	// OutputDescription describes the purpose of this output
	OutputDescription = "Data output"

	// TransactionDescription describes the purpose of this transaction
	TransactionDescription = "Create Data Transaction Example"

	// Originator specifies the originator domain or FQDN used to identify the source of the action request
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage
	Originator = "example.com"
)

// This example demonstrates how to create and send a Bitcoin transaction with an OP_RETURN data output using Alice's wallet.
func main() {
	show.ProcessStart("Create Data Transaction")
	ctx := context.Background()

	if DataToEmbed == "" {
		panic(fmt.Errorf("data to embed must be provided"))
	}

	show.Step("Alice", "Creating wallet and setting up environment")
	alice := example_setup.CreateAlice()

	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	show.Info("Data", DataToEmbed)

	// Create OP_RETURN output containing the provided data
	dataOutput, err := transaction.CreateOpReturnOutput([][]byte{[]byte(DataToEmbed)})
	if err != nil {
		panic(fmt.Errorf("failed to create OP_RETURN output: %w", err))
	}

	// Create the arguments needed for the CreateAction
	createArgs := sdk.CreateActionArgs{
		Description: TransactionDescription,
		Outputs: []sdk.CreateActionOutput{
			{
				LockingScript:     dataOutput.LockingScript.Bytes(),
				Satoshis:          0,
				OutputDescription: OutputDescription,
				Tags:              []string{"data", "example"},
			},
		},
		Labels: []string{"create_action_example"},
		Options: &sdk.CreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr(false),
		},
	}

	show.Step("Alice", "Creating transaction with OP_RETURN data")
	show.Info("Transaction description", TransactionDescription)
	show.Info("Output description", OutputDescription)

	result, err := aliceWallet.CreateAction(ctx, createArgs, Originator)
	if err != nil {
		panic(fmt.Errorf("failed to create action: %w", err))
	}

	show.WalletSuccess("CreateAction", createArgs, *result)

	txID := result.Txid.String()
	if txID == "" {
		panic(fmt.Errorf("transaction ID is empty, action creation failed"))
	}

	show.Transaction(txID)
	show.Info("Status", "Transaction successfully created and broadcast")

	if len(result.SendWithResults) > 0 {
		show.Info("Broadcast status", result.SendWithResults[0].Status)
	}
	show.ProcessComplete("Create Data Transaction")
}

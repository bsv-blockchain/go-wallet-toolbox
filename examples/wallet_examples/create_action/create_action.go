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


// TODO: need to think about how we can get a transaction first to internalize so then the user can proceed with the test
// or we can reference the faucet internalize example as a prerequsite

var (
	// DefaultRecipientAddress is the address to send satoshis to (P2PKH address)
	DefaultRecipientAddress = "1A6ut1tWnfg5mAD8s1drDLM6gNsLNGvgWq"

	// DefaultSatoshisToSend is the amount to send to the recipient
	DefaultSatoshisToSend = uint64(1000)

	// DefaultOutputDescription describes the purpose of this output
	DefaultOutputDescription = "Payment to recipient"

	// DefaultTransactionDescription describes the purpose of this transaction
	DefaultTransactionDescription = "Create action example transaction"

	// DefaultOriginator specifies the originator domain or FQDN used to identify the source of the action request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	DefaultOriginator = "example.com"

	// FundingAmount is the amount to fund Alice's wallet with (enough to cover the payment + fees)
	FundingAmount = uint64(10000)
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
			SignAndProcess: &signAndProcess, // Sign and broadcast the transaction
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

	aliceWallet, cleanup, err := alice.CreateWallet(ctx)
	defer cleanup()
	if err != nil {
		panic(fmt.Errorf("failed to create Alice's wallet: %w", err))
	}


	// TODO: should we fund this wallet or should we use the faucet example??
	// maybe its a prerequisite to use the faucet example?
	// pros : - would make the process complete for this example
	// cons: - adds complexity to the example, takes away from the main point of the example
	show.Step("Alice", fmt.Sprintf("Funding wallet with %d satoshis", FundingAmount)) 
	
	show.Info("Note", "In production, funds would come from real transactions")
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

/* Output:
🚀 STARTING: Create Action
============================================================

=== STEP ===
Alice is performing: Creating wallet and setting up environment
--------------------------------------------------
CreateWallet: 02ce33253bb3ebccf7a1a3afe38efa9a320342c89250ed4eeff08a39e1a65017d3

=== STEP ===
Alice is performing: Funding wallet with 10000 satoshis
--------------------------------------------------
Note: In production, funds would come from real transactions
Recipient address: 1A6ut1tWnfg5mAD8s1drDLM6gNsLNGvgWq

=== STEP ===
Alice is performing: Creating transaction to send 1000 satoshis
--------------------------------------------------
Transaction description: Create action example transaction
Output description: Payment to recipient

// need to complete this example output.... 
*/

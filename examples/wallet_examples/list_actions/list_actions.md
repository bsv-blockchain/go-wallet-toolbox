# List Actions Example

This example demonstrates how to retrieve a paginated list of wallet actions using the Go Wallet Toolbox SDK. Actions represent all wallet activities including transactions, payments, and other operations performed on the wallet.

## Overview

The process involves several steps:
1. Creating a wallet instance and establishing database connection.
2. Configuring pagination and filtering parameters for the action list request.
3. Calling the wallet's `ListActions` method with the configured arguments.
4. Processing and displaying the returned action data including total count and action details.
5. Using pagination parameters to manage large action histories efficiently.

This example is useful for building wallet interfaces that need to display transaction history or audit trails.

## Code Walkthrough

### Configuring Default Parameters

```go
var (
    DefaultLimit = uint32(100)
    DefaultOffset = uint32(0)
    DefaultOriginator = "example.com"
    DefaultIncludeLabels = true
)
```
The example defines default configuration values:
- **DefaultLimit**: Maximum number of actions to return per request (100)
- **DefaultOffset**: Starting position for pagination (0 = beginning)
- **DefaultOriginator**: Domain identifier for the requesting application
- **DefaultIncludeLabels**: Whether to include action labels in the response

### Creating List Arguments

```go
func defaultListActionsArgs() sdk.ListActionsArgs {
    return sdk.ListActionsArgs{
        Limit:         &DefaultLimit,
        Offset:        &DefaultOffset,
        IncludeLabels: &DefaultIncludeLabels,
    }
}
```
The `ListActionsArgs` structure controls the query behavior:
- **Limit**: Controls pagination size (how many results per page)
- **Offset**: Controls pagination position (which page to start from)
- **IncludeLabels**: Determines whether to include metadata labels with each action

### Setting Up the Wallet

```go
alice := example_setup.CreateAlice()
aliceWallet, cleanup, err := alice.CreateWallet(ctx)
defer cleanup()
```
We create Alice's wallet instance and establish a connection to the wallet database. The cleanup function ensures proper resource management when the operation completes.

### Retrieving Actions

```go
args := defaultListActionsArgs()
actions, err := aliceWallet.ListActions(ctx, args, DefaultOriginator)
```
The `ListActions` method takes:
- **Context**: For request lifecycle management
- **ListActionsArgs**: Configuration for pagination and filtering
- **Originator**: String identifying the requesting application or domain

### Processing Results

```go
show.Info("Actions", actions)
```
The response contains:
- **TotalActions**: Total number of actions available in the wallet
- **Actions**: Array of action objects with detailed information about each wallet activity

## Running the Example

To run this example:

```bash
go run ./examples/wallet_examples/list_actions/list_actions.go
```

Expected output:

```text
🚀 STARTING: List Actions
============================================================
CreateWallet: 03aeac4f9aa44ff0a8e54832415cc810d1db8367ccb33febf60cb2fa4f82b5b5c4

=== STEP ===
Alice is performing: Listing actions
--------------------------------------------------
ListActionsArgs: {Labels:[] LabelQueryMode: IncludeLabels:0x7ff621ebf380 IncludeInputs:<nil> IncludeInputSourceLockingScripts:<nil> IncludeInputUnlockingScripts:<nil> IncludeOutputs:<nil> IncludeOutputLockingScripts:<nil> Limit:0x7ff621ebf3c4 Offset:0x7ff622a29d6c SeekPermission:<nil>}
============================================================
Actions: &{TotalActions:2 Actions:[{Txid:b45178c7de8c54651f1669c3f516a0df57e2fd8ac5602f16cb17cc0c49360b40 Satoshis:99904 Status:unproven IsOutgoing:false Description:internalize from faucet Labels:[] Version:1 LockTime:0 Inputs:[] Outputs:[]} {Txid:15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8 Satoshis:1 Status:unproven IsOutgoing:false Description:internalize from faucet Labels:[] Version:1 LockTime:0 Inputs:[] Outputs:[]}]}
============================================================
🎉 COMPLETED: List Actions
```

## Integration Steps

To integrate action listing into your application:

1. **Configure pagination parameters** based on your UI requirements (page size, starting position).
2. **Set the originator identifier** to your application's domain or identifier.
3. **Choose label inclusion** based on whether you need additional metadata.
4. **Create ListActionsArgs** with your configuration parameters.
5. **Call ListActions** on your wallet instance with the arguments.
6. **Process the response** to extract total count and action details.
7. **Implement pagination logic** using offset and limit for large action histories.
8. **Handle errors** appropriately for network issues or wallet access problems.

### Pagination Example

```go
// First page
args := sdk.ListActionsArgs{
    Limit:  &[]uint32{10}[0],    // 10 items per page
    Offset: &[]uint32{0}[0],     // Start from beginning
}

// Second page
args = sdk.ListActionsArgs{
    Limit:  &[]uint32{10}[0],    // 10 items per page
    Offset: &[]uint32{10}[0],    // Skip first 10 items
}
```

### Filtering and Display

- Use `IncludeLabels: true` to get rich metadata for detailed views
- Use `IncludeLabels: false` for faster loading in simple list views
- Adjust `Limit` based on your UI's performance requirements
- Store `TotalActions` to calculate total number of pages

## Additional Resources

- [List Actions Example](./list_actions.go) - list action wallet example code

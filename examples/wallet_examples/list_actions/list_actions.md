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

### Configuration Parameters

The example uses the following configurable constants:

- **`DefaultLimit`**: Maximum number of actions to return per request (default: `100`)
- **`DefaultOffset`**: Starting position for pagination (default: `0`)
- **`DefaultOriginator`**: The originator domain or FQDN allowed to use this permission (default: `"example.com"`)
- **`DefaultIncludeLabels`**: The default value for including labels in the response (default: `true`)

### Request Parameters

The `ListActionsArgs` structure supports the following options:

- **`Limit`**: Controls how many actions to retrieve in a single request
- **`Offset`**: Specifies the starting position for pagination (useful for retrieving large action histories)
- **`IncludeLabels`**: Optional parameter to include action labels in the response

### Response Analysis

The service response contains:

- **`TotalActions`**: The total number of actions available in the wallet
- **`Actions`**: An array of action objects containing detailed information about each wallet activity

## Running the Example

To run this example:

```bash
go run ./examples/wallet_examples/list_actions/list_actions.go
```

## Expected Output

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

## Additional Resources

- [List Actions Example](./list_actions.go) - Complete code example for listing wallet actions
- [List Outputs Documentation](../list_outputs/list_outputs.md) - View wallet transaction outputs

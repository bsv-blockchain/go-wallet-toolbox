# List Failed Actions Example

This example demonstrates how to retrieve a paginated list of failed wallet actions and optionally request recovery ("unfail") in the Go Wallet Toolbox SDK. Failed actions are transactions or actions that did not complete successfully.

## Overview

The process involves several steps:
1. Creating a wallet instance and establishing database connection.
2. Configuring pagination and display parameters.
3. Calling the wallet's `ListFailedActions` method with the configured arguments and an `unfail` flag.
4. Processing and displaying the returned failed actions, including total count and details.

When `unfail` is set to true, the call requests recovery for the returned failed actions, promoting them to be reprocessed by the unfail workflow.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable constants:

- **`DefaultLimit`**: Maximum number of actions to return per request (default: `100`)
- **`DefaultOffset`**: Starting position for pagination (default: `0`)
- **`DefaultOriginator`**: The originator domain or FQDN allowed to use this permission (default: `"example.com"`)
- **`DefaultIncludeLabels`**: Whether to include labels in the response (default: `true`)
- **`DefaultUnfail`**: Whether to request promotion of the listed failed actions to "unfail" (default: `false`)

### Request Parameters

`ListFailedActions` reuses `ListActionsArgs` for pagination and inclusion flags:

- **`Limit`**: Controls how many actions to retrieve in a single request
- **`Offset`**: Specifies the starting position for pagination
- **`IncludeLabels`**: Include action labels in the response

### Behavior Notes

- The wallet injects a reserved spec-op label for "failed actions" so storage filters by status = failed.
- If `unfail = true`, a control label is also included; after fetching results, storage promotes those TXIDs for the unfail processing flow (best-effort).

## Running the Example

To run this example:

```bash
go run ./examples/wallet_examples/list_failed_actions/list_failed_actions.go
```

## Expected Output

```text
🚀 STARTING: List Failed Actions
============================================================
Using remote storage: http://localhost:8100
CreateWallet: 02a675b6767bf17f8d37755d4afb8dcea49f79c0ae696f0f59c0b38154e482520f

=== STEP ===
Alice is performing: Listing failed actions
--------------------------------------------------
ListFailedActionsArgs: {Labels:[] LabelQueryMode: IncludeLabels:0x7ff7033e34e0 IncludeInputs:<nil> IncludeInputSourceLockingScripts:<nil> IncludeInputUnlockingScripts:<nil> IncludeOutputs:<nil> IncludeOutputLockingScripts:<nil> Limit:0x7ff7033e3548 Offset:0x7ff7042fee4c SeekPermission:<nil>}
Unfail: true
============================================================
FailedActions: &{TotalActions:2 Actions:[{Txid:225cc7b2be25be0c3cc032d09fd8035128c9d8e2f0d32db6ab7d875648263bb4 Satoshis:-38 Status:failed IsOutgoing:true Description:mintPushDropToken Labels:[mintPushDropToken] Version:1 LockTime:0 Inputs:[] Outputs:[]} {Txid:568f51aa28dca612ee18351695c6a7e3f22a4a468909a5493442ed01e68e922f Satoshis:-38 Status:failed IsOutgoing:true Description:mintPushDropToken Labels:[mintPushDropToken] Version:1 LockTime:0 Inputs:[] Outputs:[]}]}
============================================================
🎉 COMPLETED: List Failed Actions
```

## Integration Steps

To integrate failed-action listing into your application:

1. **Configure pagination parameters** based on your UI needs (page size, starting offset).
2. **Set the originator identifier** to your application's domain or identifier.
3. **Choose label inclusion** if you need metadata for display or filtering.
4. **Decide on `unfail` behavior**: set to `true` to request recovery of the listed failed actions.
5. **Call `ListFailedActions`** on your wallet instance with the arguments and `unfail` flag.
6. **Handle the response** and show failed actions to the user.
7. **Monitor recovery**: when `unfail` is requested, the background unfail flow will attempt to move those actions forward.

## Additional Resources

- [List Failed Actions Code](./list_failed_actions.go) - Complete code example for listing failed actions
- [List Actions Documentation](../list_actions/list_actions.md) - General action listing doc



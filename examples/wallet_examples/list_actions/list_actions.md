# List Wallet Actions

This example demonstrates how to retrieve a paginated list of actions from a BSV wallet using the Go Wallet Toolbox SDK.

## Configuration Parameters

The example uses the following configurable constants:

- **`DefaultLimit`**: Maximum number of actions to return per request (default: `100`)
- **`DefaultOffset`**: Starting position for pagination (default: `0`)
- **`DefaultOriginator`**: Specifies the originator domain or FQDN used to identify the source of the action listing request (default: `"originator"`)
- **`DefaultIncludeLabels`**: The default value for including labels in the response (default: `true`)

## Request Parameters

The `ListActionsArgs` structure supports the following options:

- **`Limit`**: Controls how many actions to retrieve in a single request
- **`Offset`**: Specifies the starting position for pagination (useful for retrieving large action histories)
- **`IncludeLabels`**: Optional parameter to include action labels in the response

## Response Fields

The response contains:

- **`TotalActions`**: The total number of actions available for the wallet
- **`Actions`**: An array of action objects containing detailed information about each wallet activity

## Example Output

```text
🚀 STARTING: List Actions
============================================================
CreateWallet: 025dd4fd3fd0594315937f4775f11c1622581ea8372642b57864e45c7bc8b36e4f

=== STEP ===
Alice is performing: Listing actions
--------------------------------------------------
Actions: &{TotalActions:0 Actions:[]}
============================================================
🎉 COMPLETED: List Actions
```

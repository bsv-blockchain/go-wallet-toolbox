# List Wallet Actions

This example demonstrates how to retrieve a paginated list of actions from a BSV wallet using the Go Wallet Toolbox SDK.

## Configuration Parameters

The example uses the following configurable constants:

- **`DefaultLimit`**: Maximum number of actions to return per request (default: `100`)
- **`DefaultOffset`**: Starting position for pagination (default: `0`)
- **`DefaultOriginator`**: Specifies the originator domain or FQDN used to identify the source of the action listing request (default: `"example.com"`)
- **`DefaultIncludeLabels`**: The default value for including labels in the response (default: `true`)
- **`DefaultLabels`**: The default labels to filter actions by, empty means list all actions regardless of labels (default: `[]`)
- **`DefaultLabelQueryMode`**: The default mode for querying labels when multiple labels are specified (default: `QueryModeAny`)

## Request Parameters

The `ListActionsArgs` structure supports the following options:

- **`Labels`**: Filters actions by specific labels (empty array lists all actions regardless of labels)
- **`LabelQueryMode`**: Specifies how to query multiple labels - `QueryModeAny` matches actions with any of the specified labels, `QueryModeAll` matches only actions with all specified labels
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
CreateWallet: 03aeac4f9aa44ff0a8e54832415cc810d1db8367ccb33febf60cb2fa4f82b5b5c4

=== STEP ===
Alice is performing: Listing actions
--------------------------------------------------
Actions: &{TotalActions:1 Actions:[{Txid:b45178c7de8c54651f1669c3f516a0df57e2fd8ac5602f16cb17cc0c49360b40 Satoshis:99904 Status:unproven IsOutgoing:false Description:internalize from faucet Labels:[] Version:1 LockTime:0 Inputs:[] Outputs:[]}]}
============================================================
🎉 COMPLETED: List Actions
```

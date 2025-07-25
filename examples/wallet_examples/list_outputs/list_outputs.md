# List Wallet Outputs

This example demonstrates how to retrieve a paginated list of outputs from a BSV wallet using the Go Wallet Toolbox SDK.

## Configuration Parameters

The example uses the following configurable constants:

- **`DefaultLimit`**: Maximum number of outputs to return per request (default: `100`)
- **`DefaultOffset`**: Starting position for pagination (default: `0`)
- **`DefaultOriginator`**: The originator domain or FQDN allowed to use this permission (default: `"example.com"`)
- **`DefaultIncludeLabels`**: The default value for including labels in the response (default: `true`)
- **`DefaultBasket`**: The default basket to list outputs from, empty means list from all baskets (default: `""`)
- **`DefaultTags`**: The default tags to filter outputs by, empty means list all outputs regardless of tags (default: `[]`)
- **`DefaultTagQueryMode`**: The default mode for querying tags when multiple tags are specified (default: `QueryModeAny`)

## Request Parameters

The `ListOutputsArgs` structure supports the following options:

- **`Basket`**: Filters outputs by basket name (empty string lists from all baskets)
- **`Tags`**: Filters outputs by specific tags (empty array lists all outputs regardless of tags)
- **`TagQueryMode`**: Specifies how to query multiple tags - `QueryModeAny` matches outputs with any of the specified tags, `QueryModeAll` matches only outputs with all specified tags
- **`Limit`**: Controls how many outputs to retrieve in a single request
- **`Offset`**: Specifies the starting position for pagination (useful for retrieving large output histories)
- **`IncludeLabels`**: Optional parameter to include output labels in the response

## Response Fields

The response contains:

- **`TotalOutputs`**: The total number of outputs available for the wallet
- **`Outputs`**: An array of output objects containing detailed information about each wallet output

## Example Output

```text
🚀 STARTING: List Outputs
============================================================
CreateWallet: 03aeac4f9aa44ff0a8e54832415cc810d1db8367ccb33febf60cb2fa4f82b5b5c4

=== STEP ===
Alice is performing: Listing outputs
--------------------------------------------------
Outputs: &{TotalOutputs:1 BEEF:[] Outputs:[{Satoshis:99904 LockingScript:[] Spendable:true CustomInstructions: Tags:[] Outpoint:b45178c7de8c54651f1669c3f516a0df57e2fd8ac5602f16cb17cc0c49360b40.0 Labels:[]}]}
============================================================
🎉 COMPLETED: List Outputs
```

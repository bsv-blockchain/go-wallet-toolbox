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

## Request Parameters

The `ListOutputsArgs` structure supports the following options:

- **`Basket`**: Filters outputs by basket name (empty string lists from all baskets)
- **`Tags`**: Filters outputs by specific tags (empty array lists all outputs regardless of tags)
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
CreateWallet: 025dd4fd3fd0594315937f4775f11c1622581ea8372642b57864e45c7bc8b36e4f

=== STEP ===
Alice is performing: Listing outputs
--------------------------------------------------
Outputs: &{TotalOutputs:0 BEEF:[] Outputs:[]}
============================================================
🎉 COMPLETED: List Outputs
```

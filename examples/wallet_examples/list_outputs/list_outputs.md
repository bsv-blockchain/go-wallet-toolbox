# List Wallet Outputs

This example demonstrates how to retrieve a paginated list of outputs from a BSV wallet using the Go Wallet Toolbox SDK. It showcases various filtering and pagination options for managing wallet output data efficiently.

## Overview

The process involves several steps:
1. Setting up wallet configuration and establishing connection to storage.
2. Configuring request parameters including filters, pagination, and output criteria.
3. Submitting the list outputs request with specified filtering options.
4. Retrieving paginated output data with detailed information about each output.
5. Processing and displaying the wallet outputs with their associated metadata.

This approach ensures efficient output management by providing flexible filtering and pagination capabilities for wallet operations.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable constants:

- **`DefaultLimit`**: Maximum number of outputs to return per request (default: `100`)
- **`DefaultOffset`**: Starting position for pagination (default: `0`)
- **`DefaultOriginator`**: The originator domain or FQDN allowed to use this permission (default: `"example.com"`)
- **`DefaultIncludeLabels`**: The default value for including labels in the response (default: `true`)
- **`DefaultBasket`**: The default basket to list outputs from, empty means list from all baskets (default: `""`)
- **`DefaultTags`**: The default tags to filter outputs by, empty means list all outputs regardless of tags (default: `[]`)
- **`DefaultTagQueryMode`**: The default mode for querying tags when multiple tags are specified (default: `QueryModeAny`)

### Request Parameters

The `ListOutputsArgs` structure supports the following options:

- **`Basket`**: Filters outputs by basket name (empty string lists from all baskets)
- **`Tags`**: Filters outputs by specific tags (empty array lists all outputs regardless of tags)
- **`TagQueryMode`**: Specifies how to query multiple tags - `QueryModeAny` matches outputs with any of the specified tags, `QueryModeAll` matches only outputs with all specified tags
- **`Limit`**: Controls how many outputs to retrieve in a single request
- **`Offset`**: Specifies the starting position for pagination (useful for retrieving large output histories)
- **`IncludeLabels`**: Optional parameter to include output labels in the response

### Response Analysis

The service response contains:

- **`TotalOutputs`**: The total number of outputs available for the wallet
- **`Outputs`**: An array of output objects containing detailed information about each wallet output

## Running the Example

To run this example:

```bash
go run ./examples/wallet_examples/list_outputs/list_outputs.go
```

## Expected Output

```text
🚀 STARTING: List Outputs
============================================================
CreateWallet: 03aeac4f9aa44ff0a8e54832415cc810d1db8367ccb33febf60cb2fa4f82b5b5c4

=== STEP ===
Alice is performing: Listing outputs
--------------------------------------------------
ListOutputsArgs: {Basket: Tags:[] TagQueryMode:any Include: IncludeCustomInstructions:<nil> IncludeTags:<nil> IncludeLabels:0x7ff79736e380 Limit:0x7ff79736e3c4 Offset:0x7ff797ed7d6c SeekPermission:<nil>}
============================================================
Outputs: &{TotalOutputs:2 BEEF:[] Outputs:[{Satoshis:99904 LockingScript:[] Spendable:true CustomInstructions: Tags:[] Outpoint:b45178c7de8c54651f1669c3f516a0df57e2fd8ac5602f16cb17cc0c49360b40.0 Labels:[]} {Satoshis:1 LockingScript:[] Spendable:true CustomInstructions: Tags:[] Outpoint:15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8.0 Labels:[]}]}
============================================================
🎉 COMPLETED: List Outputs
```

## Integration Steps

To integrate wallet output listing into your application:

1. **Configure wallet connection** with appropriate storage and authentication settings.
2. **Set pagination parameters** including limit and offset for managing large output sets.
3. **Define filtering criteria** such as baskets, tags, and tag query modes for targeted output retrieval.
4. **Submit list outputs request** using the configured parameters and filters.
5. **Process response data** to extract total count and individual output information.
6. **Implement pagination logic** for handling large output histories across multiple requests.
7. **Handle output metadata** including labels, tags, and custom instructions as needed.

## Additional Resources

- [List Actions Documentation](../list_actions/list_actions.md) - Get wallet action history
- [List Outputs Example](./list_outputs.go) - Complete code example for listing wallet outputs

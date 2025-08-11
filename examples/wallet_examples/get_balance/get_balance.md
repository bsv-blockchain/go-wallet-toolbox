# Get Wallet Balance

This example demonstrates how to calculate the total balance of a BSV wallet by retrieving and summing all satoshis from outputs in the default basket using the Go Wallet Toolbox SDK. It showcases efficient pagination to handle wallets with large numbers of outputs.

## Overview

The process involves several steps:
1. Setting up wallet configuration and establishing connection to storage.
2. Configuring pagination parameters for efficient output retrieval.
3. Iterating through all outputs in the default basket using pagination.
4. Summing the satoshi values from all outputs to calculate total balance.
5. Displaying the final balance result.

This approach ensures efficient balance calculation by using pagination to handle wallets with any number of outputs without memory constraints.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable constants:

- **`Limit`**: Number of outputs to retrieve per page for balance calculation (default: `100`)
- **`Originator`**: The originator domain or FQDN used to identify the source of the balance request (default: `"example.com"`)
- **`Basket`**: The target basket for balance calculation - "default" holds automatically managed "change" (default: `"default"`)

### Balance Calculation Logic

The balance calculation follows this pattern:

1. **Initialize Variables**: Set up balance accumulator and pagination offset
2. **Pagination Loop**: Retrieve outputs in batches using `ListOutputs`
3. **Accumulation**: Sum satoshis from each output in the current page
4. **Continuation**: Update offset and continue until all outputs are processed
5. **Termination**: Exit when no more outputs are available or total count reached

### Request Parameters

The `ListOutputsArgs` structure is configured with:

- **`Basket`**: Set to "default" to target the automatically managed change basket
- **`Limit`**: Controls batch size for pagination (100 outputs per request)
- **`Offset`**: Tracks current position in the output list for pagination

### Balance Accumulation

For each page of outputs:
- Iterate through all outputs in the response
- Add each output's `Satoshis` value to the running balance total
- Continue until all outputs have been processed

## Running the Example

To run this example:

```bash
go run ./examples/wallet_examples/get_balance/get_balance.go
```

## Expected Output

```text
🚀 STARTING: Get Wallet Balance
============================================================
CreateWallet: 03d2276c31630d6614f65c6634f40c0735a822d3501cc403ff459200971f747970

=== STEP ===
Alice is performing: Calculating wallet balance
--------------------------------------------------
Total Balance (satoshis): 199707
============================================================
🎉 COMPLETED: Get Wallet Balance
```

## Integration Steps

To integrate wallet balance calculation into your application:

1. **Configure wallet connection** with appropriate storage and authentication settings.
2. **Set pagination parameters** including limit and offset for efficient output retrieval.
3. **Initialize balance accumulator** to track total satoshis across all outputs.
4. **Implement pagination loop** to retrieve outputs in batches from the default basket.
5. **Sum output values** by adding each output's satoshi amount to the running total.
6. **Handle pagination continuation** by updating offset and checking for completion.
7. **Return final balance** once all outputs have been processed and summed.

## Additional Resources

- [Get Balance Example](./get_balance.go) - Complete code example for calculating wallet balance
- [List Outputs Documentation](../list_outputs/list_outputs.md) - Get detailed wallet output information

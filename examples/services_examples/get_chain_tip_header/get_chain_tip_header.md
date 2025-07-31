# Find Chain Tip Header Example

This example demonstrates how to retrieve the complete block header information for the latest block (chain tip) on the BSV mainnet using blockchain data services.

## Overview

The process involves several steps:
1. Configuring the services stack with network settings and API credentials.
2. Creating a services instance with logging configuration.
3. Calling `FindChainTipHeader()` to retrieve the latest block header data.
4. Processing and displaying the complete block header information in a formatted table.

This provides comprehensive block metadata including hash, merkle root, difficulty, and timestamp information.

## Code Walkthrough

### Configuring Services

```go
cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
cfg.BHS.URL = "http://localhost:8080"
cfg.BHS.APIKey = "..." // API key for Block Headers Service
```
First, we create a default configuration for mainnet services. The Block Headers Service URL and API key are configured to access the primary blockchain data service.

### Creating Services Instance

```go
svc := services.New(slog.Default(), cfg)
```
We create a services instance with default logging and our configuration. This manages connections to blockchain data providers.

### Retrieving Chain Tip Header

```go
tip, err := svc.FindChainTipHeader(ctx)
```
The `FindChainTipHeader()` method retrieves the complete block header for the latest block on the longest chain, providing detailed blockchain metadata.

### Processing Results

```go
show.ChainTipHeaderOutput(tip)
```
The returned header contains comprehensive block information displayed in a formatted table with all relevant blockchain metadata fields.

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/find_chain_tip_header/find_chain_tip_header.go
```

Expected output:

```text
🚀 STARTING: Find Chain Tip Header
============================================================

=== STEP ===
FindChainTipHeader is performing: Finds the latest block header in the longest chain
--------------------------------------------------
✅ SUCCESS: Fetched chain tip header
Chain Tip Header:
Height  Hash                                                              Version   Prev-Hash                                                         Merkle-Root                                                       Time        Bits      Nonce
------  ----------------------------------------------------------------  --------  ---------------------------------------------------------------- ----------------------------------------------------------------  ----------  --------  ---------
905604  000000000000000005698beb20b1d7ff4ad1860314bd3c395c6db123f91c7ffd  283e2000  00000000000000000e9ee9c173a140cdc20e7f9f9f708ee276a9922c4fd6dea3  5ab8bf3278ab9d2912ade1260cacd5df9ee0b78670bbc87b9fb05a7ea5755b90  1752570909  1817a94f  342927395
============================================================
🎉 COMPLETED: Find Chain Tip Header
```

## Integration Steps

To integrate chain tip header retrieval into your application:

1. **Configure services** with appropriate network settings and API credentials.
2. **Create services instance** with logging and your configuration.
3. **Call FindChainTipHeader()** with a context for request lifecycle management.
4. **Process the response** which contains complete block header data.
5. **Extract required fields** from the header structure based on your needs.
6. **Implement error handling** for network issues or service failures.
7. **Consider caching** header data for short periods to reduce API calls.

### Response Fields

The returned block header contains:

- **Height**: The latest block height on the network
- **Hash**: The block hash (unique identifier for this block)
- **Version**: The version of the Bitcoin protocol used
- **Prev-Hash**: Hash of the previous block (links blocks in the chain)
- **Merkle-Root**: Root hash of all transactions in the block
- **Time**: UTC timestamp when the block was mined
- **Bits**: Target difficulty threshold (in compact format)
- **Nonce**: The proof-of-work solution found by miners

### Error Handling

```go
tip, err := svc.FindChainTipHeader(ctx)
if err != nil {
    // Handle service failure - implement retry logic or fallback behavior
    log.Printf("Failed to get chain tip header: %v", err)
    return err
}
```

## Additional Resources

- [Current Height Documentation](../current_height/current_height.md) - Get just the block height
- [Find Chain Tip Header Example](./find_chain_tip_header.go) - Get the complete block header information

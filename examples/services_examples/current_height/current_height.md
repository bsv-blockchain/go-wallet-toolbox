# Current Height Example

This example demonstrates how to retrieve the current block height of the BSV mainnet using a robust fallback mechanism across multiple blockchain data services.

## Overview

The process involves several steps:
1. Configuring the services stack with network settings and API credentials.
2. Creating a services instance with logging and fallback configuration.
3. Calling `CurrentHeight()` which automatically attempts multiple services in sequence.
4. Processing the returned block height representing the current chain tip.

The services stack provides automatic failover across multiple blockchain data providers to ensure reliability.

## Code Walkthrough

### Configuring Services

```go
cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
cfg.BHS.APIKey = "..." // API key for Block Headers Service
```
First, we create a default configuration for mainnet services. API credentials are configured for the primary services, with automatic fallback to alternative providers if needed.

### Creating Services Instance

```go
srv := services.New(slog.Default(), cfg)
```
We create a services instance with default logging and our configuration. This services wrapper manages the fallback logic across multiple blockchain data providers.

### Fetching Current Height

```go
height, err := srv.CurrentHeight(context.Background())
```
The `CurrentHeight()` method implements a robust fallback strategy that automatically tries multiple blockchain data services until one succeeds, returning the first valid block height obtained.

### Processing Results

```go
show.CurrentHeightOutput(height)
```
The returned height is a simple integer representing the total number of blocks currently mined on the BSV mainnet.

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/current_height/current_height.go
```

Expected output:

```text
🚀 STARTING: Get Height
============================================================

=== STEP ===
Wallet-Services is performing: fetching main-chain height (BHS → WoC → Bitails fallback)
--------------------------------------------------
2025/07/14 10:47:42 WARN error when calling service service=services.GetHeight service.name=BlockHeadersService error="failed for service BlockHeadersService: unexpected HTTP 401 for http://localhost:8080/api/v1/chain/tip/longest"
✅ SUCCESS: Fetched chain tip height

Get Height: 905465
============================================================
🎉 COMPLETED: Get Height
```

**Note**: Warning messages about individual service failures are normal and demonstrate the automatic fallback mechanism working as designed.

## Integration Steps

To integrate current height retrieval into your application:

1. **Configure services** with appropriate network settings and API credentials.
2. **Create services instance** with logging and your configuration.
3. **Call CurrentHeight()** with a context for request lifecycle management.
4. **Handle the response** which returns the current block height as a uint32.
5. **Implement error handling** for cases where all services fail.
6. **Consider caching** the result for a reasonable period to reduce API calls.
7. **Monitor service warnings** to understand which providers are experiencing issues.

### Error Handling

The services stack automatically handles individual service failures, but you should still handle the case where all services fail:

```go
height, err := srv.CurrentHeight(ctx)
if err != nil {
    // All services failed - implement retry logic or fallback behavior
    log.Printf("Failed to get current height: %v", err)
    return err
}
```

## Additional Resources

- [Current Height Example](./current_height.go) - Get current block height example code
- [Find Chain Tip Header Documentation](../find_chain_tip_header/find_chain_tip_header.md) - Get complete block header data
 
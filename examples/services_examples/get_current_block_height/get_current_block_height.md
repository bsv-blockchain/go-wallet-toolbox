# Get Current Block Height

This example demonstrates how to retrieve the current block height of the BSV blockchain using the Go Wallet Toolbox SDK. It showcases a robust fallback mechanism across multiple blockchain data services for reliable height retrieval.

## Overview

The process involves several steps:
1. Setting up services configuration with network settings and API credentials.
2. Creating a services instance with logging and fallback configuration.
3. Calling `CurrentHeight()` which automatically attempts multiple services in sequence.
4. Processing the returned block height representing the current chain tip.
5. Handling automatic failover across multiple blockchain data providers for reliability.

This approach ensures reliable access to current blockchain state through automatic service fallback and redundancy.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable settings:

- **`Network`**: Blockchain network to connect to (default: `NetworkMainnet`)
- **`BHS.APIKey`**: API key for Block Headers Service authentication (default: `"..."`)
- **`Fallback Services`**: Automatic fallback to WhatsOnChain and Bitails services when primary fails

### Service Setup

The `CurrentHeight` method requires:

- **`Context`**: Request context for lifecycle management
- **`Services Instance`**: Configured services with fallback logic across multiple providers
- **`Network Configuration`**: Mainnet settings for accessing production blockchain data

### Response Analysis

The service response contains:

- **`Block Height`**: Simple uint32 integer representing total blocks mined on the BSV blockchain
- **`Fallback Warnings`**: Automatic logging of individual service failures during fallback attempts
- **`Service Selection`**: First successful response from the configured service providers

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/get_current_block_height/get_current_block_height.go
```

## Expected Output

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

1. **Configure services** with appropriate network settings and API credentials for multiple providers.
2. **Create services instance** with logging and your configuration for automatic fallback.
3. **Submit height request** using `CurrentHeight()` with context for request lifecycle management.
4. **Process response data** to extract the current block height as a uint32 value.
5. **Handle fallback behavior** by monitoring service warnings for provider availability.
6. **Implement caching logic** for height data to reduce API calls when appropriate.
7. **Add monitoring** for service health and fallback patterns across providers.

## Additional Resources

- [Get Current Block Height Example](./get_current_block_height.go) - Complete code example for getting current block height
- [Get Chain Tip Header Documentation](../get_chain_tip_header/get_chain_tip_header.md) - Get complete block header data

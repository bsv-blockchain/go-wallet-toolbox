# Get Current Block Height

This example demonstrates how to retrieve the current block height of the BSV mainnet using a robust fallback mechanism across multiple services.

## Service Fallback Strategy

The wallet-services stack uses the following fallback approach to ensure reliability:

1. **Primary**: Block Headers Service (`/chain/tip/longest`) - Queries BHS for the current block height
2. **Secondary**: WhatsOnChain (`/chain/info`) - Falls back to WoC if BHS is unavailable
3. **Tertiary**: Bitails (`/network/info`) - Final fallback if both previous services fail
4. **Result**: Returns the first non-zero height obtained from any service

## Response

The response is a simple integer representing the current number of blocks in the main BSV chain.

- **Height**: The total number of blocks currently mined on the BSV mainnet

## Example Output

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

## Notes

The example output shows a warning when the primary BHS service returns a 401 error, demonstrating the automatic fallback to secondary services. This ensures the application continues to function even when individual services are unavailable or misconfigured.

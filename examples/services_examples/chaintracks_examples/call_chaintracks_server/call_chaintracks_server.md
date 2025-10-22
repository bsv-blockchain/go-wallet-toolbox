# Call Chaintracks Server

This example demonstrates how to call a Chaintracks server and display information from multiple service endpoints.

## Overview

The example sets up a Chaintracks client, connects to a Chaintracks server URL, and invokes several read/write methods. It shows how to configure the client for a specific network and how to call service methods using a context.

What the example does in order:

- Get general service information.
- Get current fiat exchange rates.
- Get the present blockchain height.
- Find the chain tip block hash (hex).
- Find the chain tip block header (hex fields).
- Get a header for a specific block height.
- Get a header for a specific block hash.
- Get a batch of consecutive headers starting at a height, then convert them to base headers and print each.
- Add a block header back to the server (demonstration only, re-adding the tip header retrieved earlier).

## Code Walkthrough

Key parts of the example:

- `chaintracksURL` constant — the base URL for the Chaintracks server used by the client.
- `chaintracks.NewClient(...)` — constructs a client instance with a logger and the desired network (e.g. `defs.NetworkMainnet`).
- `GetInfo(ctx)` — retrieves general server information and prints it via the project's `show` helper.
- `GetFiatExchangeRates(ctx)` — retrieves current fiat exchange rates known to the server.
- `GetPresentHeight(ctx)` — returns the server’s view of the current blockchain height.
- `FindChainTipHashHex(ctx)` — returns the best-known chain tip block hash in hex.
- `FindChainTipHeaderHex(ctx)` — returns the full chain tip header structure with hex-encoded fields.
- `FindHeaderHexForHeight(ctx, height)` — returns the header at the given block height.
- `FindHeaderHexForBlockHash(ctx, hashHex)` — returns the header for a specific block hash.
- `GetHeaders(ctx, startHeight, count)` — fetches `count` consecutive headers starting from `startHeight`.
  - The example converts the returned hashed headers to base headers using `ToBaseBlockHeaders()` and logs each one.
- `AddHeader(ctx, header.BaseBlockHeader)` — posts a block header to the server. The example re-adds the tip header it just fetched; in real use you’d post only new headers.

The example configures a context, creates the client, calls the services in the sequence above, and handles errors by terminating the program with an explanatory message if any call fails.

## Running the example

From the repository root run:

```bash
go run ./examples/services_examples/chaintracks_examples/call_chaintracks_server/call_chaintracks_server.go
```

If you use a local Chaintracks server, ensure its address matches the `chaintracksURL` constant in the example source. You can enable extra HTTP debug logs by setting the logger level (the example includes a commented line to do so).

## Expected output

The example prints a short header before each object or value returned by the Chaintracks server. Output depends on the server response and the `show` helpers. Example (illustrative):

```
Chaintracks Info: &{Chain:main HeightBulk:916909 HeightLive:918918 Storage:ChaintracksStorageNoDb BulkIngestors:[BulkIngestorCDNBabbage BulkIngestorWhatsOnChainCdn] LiveIngestors:[LiveIngestorWhatsOnChainPoll] Packages:[]}
Chaintracks Fiat Exchange Rates: &{...rates...}
Chaintracks Present Height: 918918
Chaintracks Chain Tip Hash: 0000000000000000000abc1234def...
Chaintracks Chain Tip Header: &{Version:... PrevBlock:... MerkleRoot:... Time:... Bits:... Nonce:... Height:918918 HashHex:0000...}
Chaintracks Header for Height: 918917 &{Version:... PrevBlock:... MerkleRoot:... Time:... Bits:... Nonce:... Height:918917 HashHex:0000...}
Chaintracks Header for Block Hash: 0000000000000000000abc1234def... &{Version:... PrevBlock:... MerkleRoot:... Time:... Bits:... Nonce:... Height:... HashHex:0000...}
Got 5 Chaintracks Base Headers: [...]
Header index: 0 &{Version:... PrevBlock:... MerkleRoot:... Time:... Bits:... Nonce:... Height:... Hash:...}
Header index: 1 &{...}
Header index: 2 &{...}
Header index: 3 &{...}
Header index: 4 &{...}
Successfully added Chaintracks Header: &{Version:... PrevBlock:... MerkleRoot:... Time:... Bits:... Nonce:... Height:918917 HashHex:0000...}
```

## Troubleshooting

- Connection refused: verify the Chaintracks server is running and the `chaintracksURL` is correct.
- Timeout / network errors: check firewall and network configuration.
- 404/Not Implemented: your server may not support all endpoints used in this example; update your server or skip the unsupported calls.
- Height out of range when calling `GetHeaders`: ensure `startHeight` and `count` reference headers that your server has indexed.
- Add header failures: some servers may restrict posting headers; ensure writes are enabled and the network (e.g., mainnet/testnet) matches the header being posted.
- If you need additional logging for HTTP requests, enable debug logging in the example (the example includes a commented line to set the logger level).

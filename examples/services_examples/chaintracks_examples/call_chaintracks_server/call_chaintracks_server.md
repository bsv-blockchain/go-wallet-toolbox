# Call Chaintracks Server

This example demonstrates how to call a Chaintracks server and display the service information returned by the server.

## Overview

The example sets up a Chaintracks client, connects to a Chaintracks server URL, retrieves general service information, and prints it to stdout. It shows how to configure the client for a specific network and how to invoke service methods using a context.

## Code Walkthrough

Key parts of the example:

- `chaintracksURL` constant — the base URL for the Chaintracks server used by the client.
- `chaintracks.NewClient(...)` — constructs a client instance with a logger and the desired network (e.g. `defs.NetworkMainnet`).
- `GetInfo(ctx)` — performs the call to the server to retrieve the service information and returns a structured response which the example prints using the project's `show` helper.

The example configures a context, creates the client, calls the service, and handles errors by terminating the program with an explanatory message if the call fails.

## Running the example

From the repository root run:

```bash
go run ./examples/services_examples/chaintracks_examples/call_chaintracks_server/call_chaintracks_server.go
```

If you use a local Chaintracks server, ensure its address matches the `chaintracksURL` constant in the example source.

## Expected output

The example prints a short header and the information object returned by the Chaintracks server. Output format depends on the server response and the `show.Info` helper used in the example. Example (illustrative):

```
Chaintracks Info: &{Chain:main HeightBulk:916909 HeightLive:918918 Storage:ChaintracksStorageNoDb BulkIngestors:[BulkIngestorCDNBabbage BulkIngestorWhatsOnChainCdn] LiveIngestors:[LiveIngestorWhatsOnChainPoll] Packages:[]}
```

## Troubleshooting

- Connection refused: verify the Chaintracks server is running and the `chaintracksURL` is correct.
- Timeout / network errors: check firewall and network configuration.
- If you need additional logging for HTTP requests, enable debug logging in the example (the example includes a commented line to set the logger level).


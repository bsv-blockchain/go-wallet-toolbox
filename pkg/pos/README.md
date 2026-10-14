# Merchant POS gateway

`pkg/pos` implements the signed point-of-sale protocol used by the separate Expo app in BSV Wallet. `cmd/pos` supplies an HTTP gateway using a full Go Wallet Toolbox signing wallet backed by an existing cloud storage provider.

## Run

Provision the existing wallet-toolbox cloud storage server for your merchant and configure its blockchain services, BEEF/script validation and background broadcast monitor. Provision the merchant's receiving identity into the gateway from a deployment secret store; do not provide it to cashiers.

```sh
export POS_MERCHANT_KEY_FILE=/run/secrets/merchant-private-key
export POS_STORAGE_URL=https://wallet-storage.example
export POS_PUBLIC_URL=https://merchant.example
export POS_MERCHANT_NAME='Corner Cafe'
export POS_CHAIN=test
export POS_STATE_FILE=/var/lib/bsv-pos/state.json
# Default listener: 127.0.0.1:8787; terminate TLS at a reverse proxy.
go run ./cmd/pos
```

The key file contains the merchant's hexadecimal private key. For deployment, mount it from the secret manager with restricted permissions. `POS_CHAIN` supports `test` (default) and `main`. The app's merchant administrator must use the corresponding identity and network. The server's merchant signing wallet authenticates to the storage adapter through the existing Go wallet-toolbox storage client.

No service or merchant key is deployed by this implementation. First perform testnet device pairing and payment acceptance against your provisioned services.

## Embed in another server

Construct `pos.New(pos.Config{Wallet: merchantWallet, Merchant: identity, Name: name, URL: publicHTTPSURL, Chain: "test", Store: pos.FileStore{Path: statePath}, Accept: pos.MerchantAcceptor(merchantWallet)})` and mount `service.Handler()` at `/rpc`. The wallet must provide merchant derivation, signature creation, internalization and action status querying. A storage-only provider is not sufficient.

The reference `FileStore` and service mutex support **one process per merchant**. Do not run multiple gateway processes against the same state file. Multi-instance production hosting requires a database-backed store plus a merchant-wide transactional/distributed serialization boundary that also covers wallet acceptance. Apply deployment-level request/rate limits. Retain the state file across restarts and back it up; losing it loses invoice and receipt deduplication state.

## Contract

Requests are JSON `{payload, identityKey, signature}`. The payload contains `version: 1`, `method`, a random 16-byte lowercase hex `nonce`, a millisecond Unix `time`, and `body`. Signatures use BRC-100 protocol `[2, "bsv pos authorization"]`, key ID `v1`, counterparty `anyone`. Signing bytes are UTF-8 JSON with recursively sorted field names, no HTML escaping and escaped Unicode line separators; the protocol uses ASCII field names and safe integer numeric values. DER signatures and transaction bytes are numeric JSON arrays, never base64 byte fields.

Methods:

- `info`: merchant identity, name, chain and the caller's enrollment state.
- `invite`, `cashiers`, `approve`, `revoke`: merchant-administrator operations.
- `enroll`: authenticated cashier requests admission using an unexpired invitation.
- `invoice`: active cashier submits a merchant-destination `bsvpay1` session with positive satoshi amount; service returns a signed five-minute payment request.
- `settle`: active cashier submits `{invoice, frame}`. Invoice ownership, frame kind/version, sender, derivation nonces and byte bounds are checked. Persisted original bytes cannot be replaced by another payment.
- `status`: returns the existing receipt or pending state for the caller's invoice.
- `sales`: merchant sees all sales; active cashiers see only their own sales.

The Expo app encodes signed enrollment/invoice envelopes as base64url JSON prefixed with `bsvpos1:`. The underlying session retains the existing `bsvpay1:` protocol and its encrypted radio/QR framing. No merchant keys cross to the cashier or customer.

## Acceptance policy

`MerchantAcceptor` derives the merchant's exact BRC-29 output for the supplied sender, checks its amount and script, and calls Go Wallet Toolbox `InternalizeAction`. The existing storage services perform BEEF/SPV and script validation. The result must report `Accepted`. It then queries the labeled merchant action and requires `unproven` or `completed` status. Pending broadcasts and wallet refusals never generate a signed acceptance receipt.

This is the wallet's network-acceptance policy, not block-confirmed finality. Revocation immediately stops cashier RPC authorization. Merchant administrators can inspect outstanding sales; operator recovery for a revoked cashier's unsettled invoice is not automated in this version.

A failed response is uncertain. Preserve the payment and use fresh authenticated RPC requests with the same invoice and transaction. The service issues the same stored signed receipt on repeat delivery and verifies that replayed RPC nonces cannot be reused. A crash after wallet acceptance but before receipt persistence is recovered through idempotent internalization of the exact original transaction. Invalid transactions remain non-paid and may need operator intervention; the first version does not provide a cashier override or replacement-payment flow.

## Tests

```sh
go test -race ./pkg/pos ./cmd/pos
```

Tests cover enrollment permissions, revocation, tampering, replay rejection, invoice binding, persistence across restart, concurrent submissions, destination/amount and broadcast-status gating. `testdata/typescript-signature.json` is generated with the Expo protocol and verified by the Go implementation; deterministic Go signature bytes match the TypeScript vector.

## Container release

`Dockerfile.pos` builds the dedicated `cmd/pos` executable. The **Build and Push Merchant POS Image** workflow publishes `ghcr.io/bsv-blockchain/go-wallet-toolbox-pos` on version tags or manual dispatch, with an explicit version and `sha-<commit>` tag. It runs POS race tests before publishing. Never deploy `latest`; use the published digest.

The container runs as UID/GID 1000, listens on port 8787, and persists state in `/app/data/pos-state.json`. Mount the merchant key as a read-only secret file and the data directory on a durable writable volume. Run exactly one replica with a Recreate strategy. `GET /healthz` reports process health; it does not certify storage or blockchain availability. Only a validated signed acceptance receipt confirms a payment.

The Flux pilot lives in `bsva-infra-flux`, namespace `merchant-pos`, on the US cluster and testnet. Its own merchant key is provisioned through AWS SSM and External Secrets; never reuse the storage server identity. Deployment changes require a reviewed infrastructure PR.

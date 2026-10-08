# BSV Certifier Server (example)

A runnable example of a **BRC-100 certifier wallet service** built on the
`bsv-blockchain/go-wallet-toolbox` and `bsv-blockchain/go-sdk` packages.

A user wallet calls the server with an encrypted request for a new certificate.
The server decrypts the request fields, validates them, re-issues the
certificate signed by the certifier identity stored in `config.yaml`, and
returns the signed certificate to the caller.

The example mirrors the certifier flow required by wallet applications on BSV:
the certifier is the trusted party that asserts a subject's identity fields.

## Requirements

- Go 1.27 or newer (`go.mod` declares `go 1.27.0`).
- A wallet storage backend:
  - **Local SQLite** — used automatically when `storage.url` is empty, or
  - **Remote storage server** — the go-wallet-toolbox storage server
    (JSON-RPC with Authrite authentication, see
    [`docs/storage_server.md`](https://github.com/bsv-blockchain/go-wallet-toolbox/blob/main/docs/storage_server.md)),
    reachable at the URL in `storage.url`.

No BSV node is required for the signing flow itself; broadcasting signed
certificates is outside the scope of this example.

## Configuration

The server reads `config.yaml` from the example directory. Create it from the
template:

```sh
cp config.example.yaml config.yaml
```

| Key | Example | Description |
|-----|---------|-------------|
| `server.port` | `3000` | Port to listen on. Default `3000` if empty. |
| `server.network` | `test` | Network: `test`, `stn` or `main`. Defaults to `test`. |
| `certifier_wallet.identity_key` | `03a6f9...` | Compressed public key (DER hex, 33 bytes) of the certifier wallet. Must equal the public key derived from `private_key`; the server verifies this at startup. |
| `certifier_wallet.private_key` | `7180af...` | Private key (hex, 32 bytes) of the certifier wallet. |
| `user_wallet.identity_key` | `02764e...` | Public key of the caller (user) wallet. Must match `user_wallet.private_key`. |
| `user_wallet.private_key` | `f58bf3...` | Private key of the caller wallet. |
| `storage.url` | `http://localhost:8100` | Remote storage server URL. Leave empty to use local SQLite (file `storage.sqlite` in this directory). |
| `storage.private_key` | `2b32d4...` | Server private key used to derive the storage identity key. Required. |

The keys in `config.example.yaml` are committed test keys for `test` network —
do not reuse them outside development.

## Run

```sh
go run ./cmd/server
```

On startup the server:

1. Loads and validates `config.yaml` (identity keys must match their private keys).
2. Wires the certifier wallet to the configured storage backend.
3. Listens on `server.port`.

A `GET /` returns `405 Method not allowed` — the server serves a single `POST /`
route, so any response other than `000`/connection-refused proves the process is
up and routed:

```sh
curl -i http://localhost:3000/
# HTTP/1.1 405 Method not allowed
```

## Endpoint: `POST /`

The only route. Request body is a serialized
[`certificates.MasterCertificate`](https://pkg.go.dev/github.com/bsv-blockchain/go-sdk/auth/certificates#MasterCertificate)
JSON document produced by the user wallet (encrypted fields plus the master
keyring).

The handler:

1. Validates the request: type must be `TestType` (`constants.SupportedCertType`),
   fields and master keyring must be present.
2. Decrypts the fields using the user wallet's private key from config.
3. Validates decrypted fields — `Email`, `FirstName` and `LastName` are required.
4. Issues a certificate for the subject signed by the certifier wallet.
5. Returns the signed certificate as binary (BEEF) in the response body.

## Trying it end-to-end: `test_sign_certificate`

The `test_sign_certificate` helper builds a certificate request from the
`user_wallet` in `config.yaml` and posts it to the server. Start the server in
one terminal, then run:

```sh
go run ./test_sign_certificate
```

It reports the server response for the signed certificate.
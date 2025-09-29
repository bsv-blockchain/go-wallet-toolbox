# BSV BLOCKCHAIN | Wallet Toolbox for Go

Welcome to the BSV Blockchain Wallet Toolbox for Go — BRC-100 conforming wallet components providing wallet, storage, services, and a minimal storage server. Built on top of the official [Go SDK](https://github.com/bsv-blockchain/go-sdk), this toolbox helps you assemble scalable wallet-backed applications and services.

## Table of Contents

- [Objective](#objective)
- [Getting Started](#getting-started)
  - [Installation](#installation)
  - [Quick Start: Storage Server](#quick-start-storage-server)
  - [Examples & Usage Guides](#examples--usage-guides)
- [Building Blocks](#building-blocks)
- [Features](#features)
- [Documentation](#documentation)
- [Contribution Guidelines](#contribution-guidelines)
- [Support & Contacts](#support--contacts)
- [License](#license)

## Objective

Provide interlocking, production-ready building blocks for BSV wallet applications: persistent storage, protocol-based key derivation, wallet orchestration, and integrations with blockchain services. The toolbox complements the lower-level primitives in the Go SDK, enabling SPV-friendly, privacy-preserving, scalable wallet workflows.

## Getting Started

### Installation

```bash
go get github.com/bsv-blockchain/go-wallet-toolbox
```

### Quick Start: Storage Server

Run a local storage server for development/testing.

1) Generate a config (or copy `infra-config.example.yaml`):

```bash
go run ./cmd/infra_config_gen -k
```

2) Start the server:

```bash
go run ./cmd/infra
```

Defaults:
- HTTP listens on port `8100` (see `infra-config.example.yaml`)
- SQLite at `./storage.sqlite` by default

For a guided walkthrough (including faucet and local setup), see the examples overview below and the server notes in `examples/README.md`.

### Examples & Usage Guides

Examples live under `./examples` and are grouped by purpose:
- `wallet_examples/`: end-to-end wallet actions (create P2PKH/data tx, balance, encryption, internalize, batching).
- `services_examples/`: interactions with external services (headers, BEEF, status checks, WOC/ARC helpers, etc.).
- `complex_wallet_examples/create_faucet_server/`: a runnable faucet server with Docker support.

Start with `examples/README.md` for a step‑by‑step flow (fund via faucet → check balance → create/send transactions). Quick start for the faucet server is in `examples/complex_wallet_examples/create_faucet_server/QUICK_START.md`.

## Building Blocks

- Wallet (`pkg/wallet`): high-level wallet orchestration over SDK primitives and templates.
- Storage (`pkg/storage`): durable records for actions, outputs, users, tx notes, and related entities.
- Storage Server (`cmd/infra`, `pkg/infra`): minimal HTTP server to persist wallet actions and coordinate background tasks.
- Wallet Services (`pkg/services`): integrations for transaction broadcast, headers, proofs, exchange rates, and related service APIs.

Complementary modules you may use:
- Monitor (`pkg/monitor`): background tasks (send waiting, fail abandoned, sync statuses, etc.).
- WDK/Assembler (`pkg/wdk`, `pkg/internal/assembler`): transaction assembly helpers.

## Features

- Protocol-aligned wallet flows (BRC-100 concepts).
- Persistent, queryable wallet state (SQLite/MySQL/Postgres via GORM).
- Pluggable service layer (ARC, WOC, Bitails, BHS) with configurable credentials.
- Background tasks for SPV-friendly workflows and reliable broadcasting.
- Example-driven guidance for common wallet actions and service integrations.

## Documentation

- Core concepts and examples: `./examples/README.md`
- Complex example: `./examples/complex_wallet_examples/create_faucet_server/QUICK_START.md`
- Config template: `./infra-config.example.yaml`
- Underlying primitives: [Go SDK docs](https://pkg.go.dev/github.com/bsv-blockchain/go-sdk)
- Monitor: `./docs/monitor.md`
- Storage Server: `./docs/storage_server.md`
- Storage: `./docs/storage.md`
- Wallet: `./docs/wallet.md`

The repository is annotated with code-level docs to surface well in editors.

## Contribution Guidelines

We're always looking for contributors to help improve the toolbox.

1. **Fork & Clone**: Fork this repository and clone it locally.
2. **Set Up**: Run `go mod tidy` to install all dependencies.
3. **Make Changes**: Create a new branch and make your edits.
4. **Test**: Run `go test ./...`.
5. **Commit**: Commit and push to your fork.
6. **Pull Request**: Open a PR to this repository. See [CONTRIBUTING.md](./CONTRIBUTING.md).

## Support & Contacts

For questions, bug reports, or feature requests, please open an issue on GitHub.

## License

Open BSV License. See [`LICENSE`](./LICENSE).

Thank you for being a part of the BSV Blockchain Libraries Project. Let's build the future of BSV Blockchain together!

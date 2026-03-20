<div align="center">

# 🧰&nbsp;&nbsp;go-wallet-toolbox

**BSV wallet toolbox for blockchain interactions and wallet management.**

<br/>

<a href="https://github.com/bsv-blockchain/go-wallet-toolbox/releases"><img src="https://img.shields.io/github/release-pre/bsv-blockchain/go-wallet-toolbox?include_prereleases&style=flat-square&logo=github&color=black" alt="Release"></a>
<a href="https://golang.org/"><img src="https://img.shields.io/github/go-mod/go-version/bsv-blockchain/go-wallet-toolbox?style=flat-square&logo=go&color=00ADD8" alt="Go Version"></a>
<a href="LICENSE"><img src="https://img.shields.io/badge/license-OpenBSV-blue?style=flat-square&logo=springsecurity&logoColor=white" alt="License"></a>

<br/>

<table align="center" border="0">
  <tr>
    <td align="right">
       <code>CI / CD</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/bsv-blockchain/go-wallet-toolbox/actions"><img src="https://img.shields.io/github/actions/workflow/status/bsv-blockchain/go-wallet-toolbox/fortress.yml?branch=main&label=build&logo=github&style=flat-square" alt="Build"></a>
       <a href="https://github.com/bsv-blockchain/go-wallet-toolbox/actions"><img src="https://img.shields.io/github/last-commit/bsv-blockchain/go-wallet-toolbox?style=flat-square&logo=git&logoColor=white&label=last%20update" alt="Last Commit"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Quality</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://goreportcard.com/report/github.com/bsv-blockchain/go-wallet-toolbox"><img src="https://goreportcard.com/badge/github.com/bsv-blockchain/go-wallet-toolbox?style=flat-square" alt="Go Report"></a>
       <a href="https://codecov.io/gh/bsv-blockchain/go-wallet-toolbox"><img src="https://codecov.io/gh/bsv-blockchain/go-wallet-toolbox/branch/main/graph/badge.svg?style=flat-square" alt="Coverage"></a>
    </td>
  </tr>

  <tr>
    <td align="right">
       <code>Security</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://scorecard.dev/viewer/?uri=github.com/bsv-blockchain/go-wallet-toolbox"><img src="https://api.scorecard.dev/projects/github.com/bsv-blockchain/go-wallet-toolbox/badge?style=flat-square" alt="Scorecard"></a>
       <a href=".github/SECURITY.md"><img src="https://img.shields.io/badge/policy-active-success?style=flat-square&logo=security&logoColor=white" alt="Security"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Community</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/bsv-blockchain/go-wallet-toolbox/graphs/contributors"><img src="https://img.shields.io/github/contributors/bsv-blockchain/go-wallet-toolbox?style=flat-square&color=orange" alt="Contributors"></a>
       <a href="https://github.com/sponsors/bsv-blockchain"><img src="https://img.shields.io/badge/sponsor-BSV-181717.svg?logo=github&style=flat-square" alt="Sponsor"></a>
    </td>
  </tr>
</table>

</div>

<br/>
<br/>

<div align="center">

### <code>Project Navigation</code>

</div>

<table align="center">
  <tr>
    <td align="center" width="33%">
       📦&nbsp;<a href="#-installation"><code>Installation</code></a>
    </td>
    <td align="center" width="33%">
       🧪&nbsp;<a href="#-examples--tests"><code>Examples&nbsp;&&nbsp;Tests</code></a>
    </td>
    <td align="center" width="33%">
       📚&nbsp;<a href="#-documentation"><code>Documentation</code></a>
    </td>
  </tr>
  <tr>
    <td align="center">
       🤝&nbsp;<a href="#-contributing"><code>Contributing</code></a>
    </td>
    <td align="center">
       🛠️&nbsp;<a href="#-code-standards"><code>Code&nbsp;Standards</code></a>
    </td>
    <td align="center">
       ⚡&nbsp;<a href="#-benchmarks"><code>Benchmarks</code></a>
    </td>
  </tr>
  <tr>
    <td align="center">
       🤖&nbsp;<a href="#-ai-usage--assistant-guidelines"><code>AI&nbsp;Usage</code></a>
    </td>
    <td align="center">
       ⚖️&nbsp;<a href="#-license"><code>License</code></a>
    </td>
    <td align="center">
       👥&nbsp;<a href="#-maintainers"><code>Maintainers</code></a>
    </td>
  </tr>
</table>
<br/>

## 📖 About

Welcome to the BSV Blockchain Wallet Toolbox for Go — a BRC-100 conforming collection of wallet components that provide storage, services, and a minimal storage server, all built on top of the official [Go SDK](https://github.com/bsv-blockchain/go-sdk). This toolbox gives you everything you need to assemble scalable, production-ready wallet-backed applications and services.

The toolbox provides interlocking, production-ready building blocks for BSV wallet applications: persistent storage, protocol-based key derivation, wallet orchestration, and seamless integrations with blockchain services. By complementing the lower-level primitives in the Go SDK, it enables SPV-friendly, privacy-preserving, and scalable wallet workflows.

### Features
- Protocol-aligned wallet flows (BRC-100 concepts).
- Persistent, queryable wallet state (SQLite/MySQL/Postgres via GORM).
- Pluggable service layer (ARC, WOC, Bitails, BHS) with configurable credentials.
- Background tasks for SPV-friendly workflows and reliable broadcasting.
- Example-driven guidance for common wallet actions and service integrations.

<br/>

## 📦 Installation

**go-wallet-toolbox** requires a [supported release of Go](https://golang.org/doc/devel/release.html#policy).
```shell script
go get -u github.com/bsv-blockchain/go-wallet-toolbox
```

<br/>

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

<br>

## 📚 Documentation
- Core concepts and examples: `./examples/README.md`
- Complex example: `./examples/complex_wallet_examples/create_faucet_server/QUICK_START.md`
- Config template: `./infra-config.example.yaml`
- Underlying primitives: [Go SDK docs](https://pkg.go.dev/github.com/bsv-blockchain/go-sdk)
- Monitor: `./docs/monitor.md`
- Storage Server: `./docs/storage_server.md`
- Storage: `./docs/storage.md`
- Wallet: `./docs/wallet.md`

<br/>

### Examples & Usage Guides

Examples live under [`./examples`](/examples) and are grouped by purpose:
- `wallet_examples/`: end-to-end wallet actions (create P2PKH/data tx, balance, encryption, internalize, batching).
- `services_examples/`: interactions with external services (headers, BEEF, status checks, WOC/ARC helpers, etc.).
- `complex_wallet_examples/create_faucet_server/`: a runnable faucet server with Docker support.

Start with `examples/README.md` for a step‑by‑step flow (fund via faucet → check balance → create/send transactions). Quick start for the faucet server is in `examples/complex_wallet_examples/create_faucet_server/QUICK_START.md`.

### Building Blocks

- Wallet (`pkg/wallet`): high-level wallet orchestration over SDK primitives and templates.
- Storage (`pkg/storage`): durable records for actions, outputs, users, tx notes, and related entities.
- Storage Server (`cmd/infra`, `pkg/infra`): minimal HTTP server to persist wallet actions and coordinate background tasks.
- Wallet Services (`pkg/services`): integrations for transaction broadcast, headers, proofs, exchange rates, and related service APIs.

Complementary modules you may use:
- Monitor (`pkg/monitor`): background tasks (send waiting, fail abandoned, sync statuses, etc.).
- WDK/Assembler (`pkg/wdk`, `pkg/internal/assembler`): transaction assembly helpers.

<br>

<details>
<summary><strong><code>Development Build Commands</code></strong></summary>
<br/>

Get the [MAGE-X](https://github.com/mrz1836/mage-x) build tool for development:
```shell script
go install github.com/mrz1836/mage-x/cmd/magex@latest
```

View all build commands

```bash script
magex help
```

</details>

<details>
<summary><strong><code>Library Deployment</code></strong></summary>
<br/>

This project uses [goreleaser](https://github.com/goreleaser/goreleaser) for streamlined binary and library deployment to GitHub. To get started, install it via:

```bash
brew install goreleaser
```

The release process is defined in the [.goreleaser.yml](.goreleaser.yml) configuration file.


Then create and push a new Git tag using:

```bash
magex version:bump push=true bump=patch branch=main
```

This process ensures consistent, repeatable releases with properly versioned artifacts and citation metadata.

</details>

<details>
<summary><strong><code>Pre-commit Hooks</code></strong></summary>
<br/>

Set up the Go-Pre-commit System to run the same formatting, linting, and tests defined in [AGENTS.md](.github/AGENTS.md) before every commit:

```bash
go install github.com/mrz1836/go-pre-commit/cmd/go-pre-commit@latest
go-pre-commit install
```

The system is configured via [`.github/env/`](.github/env/README.md) and provides 17x faster execution than traditional Python-based pre-commit hooks. See the [complete documentation](http://github.com/mrz1836/go-pre-commit) for details.

</details>

<details>
<summary><strong><code>GitHub Workflows</code></strong></summary>
<br/>

All workflows are driven by modular configuration in [`.github/env/`](.github/env/README.md) — no YAML editing required.

**[View all workflows and the control center →](.github/docs/workflows.md)**

</details>

<details>
<summary><strong><code>Updating Dependencies</code></strong></summary>
<br/>

To update all dependencies (Go modules, linters, and related tools), run:

```bash
magex deps:update
```

This command ensures all dependencies are brought up to date in a single step, including Go modules and any tools managed by [MAGE-X](https://github.com/mrz1836/mage-x). It is the recommended way to keep your development environment and CI in sync with the latest versions.

</details>

<br/>

## 🧪 Examples & Tests

All unit tests and [examples](examples) run via [GitHub Actions](https://github.com/bsv-blockchain/go-wallet-toolbox/actions) and use [Go version 1.26.x](https://go.dev/doc/go1.26). View the [configuration file](.github/workflows/fortress.yml).

Run all tests (fast):

```bash script
magex test
```

Run all tests with race detector (slower):
```bash script
magex test:race
```

<br/>

## ⚡ Benchmarks

Run the Go benchmarks:

```bash script
magex bench
```

<br/>

## 🛠️ Code Standards
Read more about this Go project's [code standards](.github/CODE_STANDARDS.md).

<br/>

## 🤖 AI Usage & Assistant Guidelines
Read the [AI Usage & Assistant Guidelines](.github/tech-conventions/ai-compliance.md) for details on how AI is used in this project and how to interact with AI assistants.

<br/>

## 👥 Maintainers
| [<img src="https://github.com/icellan.png" height="50" alt="Siggi" />](https://github.com/icellan) | [<img src="https://github.com/galt-tr.png" height="50" alt="Galt" />](https://github.com/galt-tr) | [<img src="https://github.com/mrz1836.png" height="50" alt="MrZ" />](https://github.com/mrz1836) |
|:--------------------------------------------------------------------------------------------------:|:-------------------------------------------------------------------------------------------------:|:------------------------------------------------------------------------------------------------:|
|                                [Siggi](https://github.com/icellan)                                 |                                [Dylan](https://github.com/galt-tr)                                |                                [MrZ](https://github.com/mrz1836)                                 |

<br/>

## 🤝 Contributing
View the [contributing guidelines](.github/CONTRIBUTING.md) and please follow the [code of conduct](.github/CODE_OF_CONDUCT.md).

### How can I help?
All kinds of contributions are welcome :raised_hands:!
The most basic way to show your support is to star :star2: the project, or to raise issues :speech_balloon:.

[![Stars](https://img.shields.io/github/stars/bsv-blockchain%2Fgo-wallet-toolbox?label=Please%20like%20us&style=social&v=1)](https://github.com/bsv-blockchain/go-wallet-toolbox/stargazers)

<br/>

## 📝 License

[![License](https://img.shields.io/badge/license-OpenBSV-blue?style=flat&logo=springsecurity&logoColor=white)](LICENSE)

# BSV Wallet Toolbox for Go

A comprehensive Go toolkit for building BSV blockchain applications with BRC-100 compliant wallet functionality, persistent storage, and network services.

## Overview

The BSV Wallet Toolbox provides essential building blocks for developing sophisticated BSV blockchain applications. Built on top of the [BSV Go SDK](https://github.com/bsv-blockchain/go-sdk), it offers:

- **BRC-100 Compliant Wallet** - Full-featured wallet with transaction creation, signing, and management
- **Persistent Storage** - GORM-based database storage for wallet data and transaction history
- **Network Services** - Integration with BSV services like ARC, WhatsOnChain, and Block Header Services
- **BRC-29 Protocol** - Private key derivation and address creation following BRC-29 standard
- **Storage Server** - Ready-to-deploy storage server infrastructure

## Key Features

### Wallet
- Create and manage BRC-100 compliant wallets
- Generate addresses and manage key derivation
- Create, sign, and broadcast transactions
- Handle UTXOs and transaction outputs
- Support for BEEF (BSV Extended Format) transactions

### Storage
- Database-backed wallet storage using GORM
- Support for MySQL, PostgreSQL, and SQLite
- Transaction synchronization and status tracking
- User management and authorization
- Configurable commission and fee models

### Services
- Transaction broadcasting via ARC and other BSV services
- Blockchain data retrieval (merkle paths, headers, transaction history)
- Multiple service provider support with automatic failover
- Exchange rate services for fiat currency conversion

### BRC-29 Protocol
- Generate BRC-29 compliant addresses
- Private key derivation using BRC-29 standard
- Support for both sender and recipient key management

## Quick Start

### Installation

```bash
go get github.com/bsv-blockchain/go-wallet-toolbox
```

### Basic Wallet Usage

```go
package main

import (
    "context"
    "log/slog"
    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
    sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func main() {
    ctx := context.Background()
    logger := slog.Default()
    
    // Create storage provider
    storageProvider, err := storage.NewGORMProvider(logger, storage.GORMProviderConfig{
        Chain: defs.NetworkTestnet,
        DB:    dbConfig,
        // ... other config
    })
    if err != nil {
        panic(err)
    }
    
    // Create wallet with private key
    walletInstance, err := wallet.New(defs.NetworkTestnet, privateKey, storageProvider)
    if err != nil {
        panic(err)
    }
    
    // Generate BRC29 address
    address, err := brc29.Address(privateKey, keyID, identityKey, brc29.WithTestNet())
    if err != nil {
        panic(err)
    }
    
    // Create transaction
    result, err := walletInstance.CreateAction(ctx, sdk.CreateActionArgs{
        Outputs: []sdk.CreateActionOutput{
            {
                Script:      lockingScript,
                Satoshis:    1000,
                Description: "Payment description",
            },
        },
        Description: "My transaction",
    }, "originator")
    if err != nil {
        panic(err)
    }
    
    // Sign the transaction
    signedResult, err := walletInstance.SignAction(ctx, sdk.SignActionArgs{
        Ref:    result.Ref,
        Spends: result.Spends,
    }, "originator")
    if err != nil {
        panic(err)
    }
}
```

## Examples

The `/examples` directory contains comprehensive examples demonstrating the key functionality:

### Wallet Examples
- **faucet_address** - Generate testnet addresses for funding from faucets
- **faucet_internalize** - Internalize transactions from testnet faucets into wallet

### Services Examples  
- **services_post_beef** - Broadcast transactions using BEEF format
- **services_merkle_path** - Retrieve merkle paths for transactions
- **services_post_multiple_txs** - Handle multiple transaction broadcasts
- **find_chain_tip_header** - Get current blockchain tip information

Check the `/examples` directory for complete, runnable examples of all major features.

## Storage Server

The toolbox includes a ready-to-deploy storage server for wallet infrastructure:

### Running the Storage Server

```bash
# Copy and configure the example config
cp infra-config.example.yaml infra-config.yaml

# Edit infra-config.yaml with your settings
# Then run the server
go run cmd/infra/main.go
```

The storage server provides:
- RESTful API for wallet operations
- Database management and migrations
- User authentication and authorization
- Transaction synchronization services
- Monitoring and health checks

### Configuration

The server uses YAML configuration files. See `infra-config.example.yaml` for available options including:
- Database connection settings
- Service provider configurations
- Fee and commission models
- Network settings (testnet/mainnet)

## Documentation

The codebase is extensively documented with godoc-compatible comments. View documentation with:

```bash
go doc github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet
go doc github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage
go doc github.com/bsv-blockchain/go-wallet-toolbox/pkg/services
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on:
- Setting up the development environment
- Running tests and linting
- Submitting pull requests
- Code standards and practices

## License

This project is licensed under the Open BSV License. See [LICENSE](./LICENSE) for details.

---

**BSV Blockchain Libraries Project** - Building the future of BSV blockchain development

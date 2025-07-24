# Create Action Example

This example demonstrates how to create and send a Bitcoin transaction using the wallet's `CreateAction` method.

## Configuration

The example uses these default values (configurable at the top of the file):

- **Recipient Address**: `1A6ut1tWnfg5mAD8s1drDLM6gNsLNGvgWq` (testnet address)
- **Amount to Send**: 1000 satoshis
- **Transaction Description**: "Create action example transaction"
- **Output Description**: "Payment to recipient"

## Prerequisites

For this example to work with real funds, you would need follow the steps in the [setup example]() // add link once main README PR is merged

1. A funded wallet with sufficient UTXOs


## Running the Example

```bash
go run examples/wallet_examples/create_action/create_action.go
```

Note: This example will fail if the wallet doesn't have sufficient funds. In a real application, you would need to fund the wallet first using other transactions or faucet services.
// add reference to faucet examples


// add logs 

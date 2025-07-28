# Faucet – Internalize

In this section, we’ll finish the example by **internalizing** a Testnet transaction on your local Wallet Toolbox server.
<br>**Note**:💡 This example is created as a helper to get funds into the wallet so you can use other examples.

## 0. Prerequisite: Transaction ID from the faucet.
In the [previous step](../faucet_address/README) you received a **txid** from a public Testnet faucet. If you have not completed the Faucet Address step yet, please refer [here](../faucet_address/README.md) before continuing.

## 1. Copy the TxID
Open `faucet_internalize.go` and replace the placeholder with your own value:

```go
// The txid is the transaction id of the transaction to internalize
// Pass the chosen txid or simply change the default value when running the example
var txID = "15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8" // <-- replace me
```

A helper function—[`woc_get_beef_from_txid`](../../internal/utils/woc_get_beef_from_txid.go)—retrieves the full transaction **BEEF** hexadecimal ([BRC-62 spec](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0062.md)).  
That transaction is then passed to `InternalizeAction`, crediting Alice’s wallet.

## 2. Internalize the Transaction
Run the example:

```bash
go run ./examples/wallet_examples/faucet_internalize/faucet_internalize.go
```

Typical output:

```text
🚀 STARTING: Faucet Transaction Internalization
============================================================

=== STEP ===
Alice is performing: Creating wallet and setting up environment
--------------------------------------------------
CreateWallet: 020c0ca23c75f7312bad0c5d81bff858bdcf468d3ad69a60b46ae90cafef557b03

=== STEP ===
Alice is performing: Retrieving BEEF data for transaction
--------------------------------------------------
🔗 TRANSACTION:
   TxID: 15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8

=== STEP ===
Alice is performing: Internalizing transaction from faucet
--------------------------------------------------
WALLET CALL: InternalizeAction
Args: {Tx:[ …snipped hex… ]}
✅ Result: {Accepted:true}

✅ SUCCESS: Transaction internalized successfully
============================================================
🎉 COMPLETED: Faucet Transaction Internalization
```

If successful, you’ll find a new row in `storage.sqlite` under Alice’s identity key.

---

### What you just did

1. **Fetched** raw transaction data from its transaction ID.  
2. **Internalized** that transaction via `InternalizeAction`.  
3. **Verified** the result in your local database.

You can now continue with other examples [provided](../../README.md).

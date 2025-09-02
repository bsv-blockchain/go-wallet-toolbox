# NoSend + SendWith (Batch Broadcast) Example

This example shows how to construct multiple transactions without broadcasting them immediately (NoSend), chain their internal change across steps (NoSendChange), and then broadcast them together in a single batch using SendWith. The demo uses simple PushDrop "tokens" to make the flow concrete.

## Overview

The flow demonstrates:
1. Setting up a wallet connection for Alice.
2. Minting several small PushDrop outputs with `Options.NoSend=true` so they aren’t broadcast yet.
3. Passing the returned NoSendChange from each mint into the next mint so they form a single spend-chain.
4. Broadcasting all pending transactions at once using Options.SendWith.
5. (Optional) Redeeming those outputs with the same NoSend/SendWith pattern.

Batching related transactions helps reduce race conditions and improves UX by confirming a set of actions together.

## Prerequisites

- The wallet must have funds to pay fees. See the examples README for local setup and funding guidance.
- Example uses the default local configuration in examples/examples-config.yaml.

## Code Walkthrough

The main entry point is no_send_send_with.go. Key pieces:

- `tokensCount`: How many PushDrop outputs to mint (default: 3).
- `dataPrefix`: Text prefix embedded in the token’s data (e.g., `"exampletoken-0"`).
- Mint flow: For each token, CreateAction is called with:
  - Options.NoSend = true to prevent immediate broadcast.
  - Options.NoSendChange carrying the previous step’s change outpoints so the chain stays connected.
  - Output contains the PushDrop locking script and a small satoshi amount.
- Batch broadcast: After minting, CreateAction is called once more with `Options.SendWith` set to the list of prior txids. The wallet broadcasts them together.
- Optional redeem flow: Builds a spend using CreateAction (NoSend) + SignAction, threads NoSendChange again, then batch broadcasts with SendWith. It’s left commented in main for simplicity.

### Configuration Parameters

- tokensCount: Number of tokens to mint (default: 3)
- dataPrefix: Data prefix written into each token (default: "exampletoken")
- keyID: Random short identifier for the PushDrop protocol instance (auto-generated)

### Request Parameters

CreateActionArgs highlights used here:

- Outputs: One PushDrop output per mint with a small satoshi value.
- Options.NoSend: When true, the transaction is constructed but not broadcast.
- Options.NoSendChange: A list of internal change outpoints produced by a previous NoSend action, passed to the next action so they can be spent immediately in-memory.
- Options.SendWith: A list of txids to broadcast together now. Used once at the end to send all staged transactions.

Redeem uses SignAction after CreateAction provides a SignableTransaction to finalize the unlocking script.

### Response Notes

- CreateAction returns NoSendChange when Options.NoSend is true. Save it to feed into the next step.
- When using Options.SendWith, CreateAction returns SendWithResults that include per-tx broadcast status.
- For redeem, CreateAction may return a SignableTransaction that you then complete via SignAction.

## Running the Example

```bash
go run ./examples/wallet_examples/no_send_send_with/no_send_send_with.go
```

## Expected Output

```text
🚀 STARTING: NoSend and SendWith Example based on PushDrop Tokens
============================================================
CreateWallet: 0320bbfb879bbd6761ecd2962badbb41ba9d60ca88327d78b07ae7141af6b6c810

=== STEP ===
Mint multiple tokens is performing: all mints are done with noSend = true, so they are not broadcasted immediately
--------------------------------------------------
Mint token, Locking Script: 210211b8b66bda6bcf8538bc847dda0989530ceffc1e2e2089852006f510d58ec411ac0e6578616d706c65746f6b656e2d3075
Minted Token: fc3567d9f7dd17d7983a6d1c25ce01f0b31ad05534d2159230e8e2dcffc6c3c4
Mint token, Locking Script: 210211b8b66bda6bcf8538bc847dda0989530ceffc1e2e2089852006f510d58ec411ac0e6578616d706c65746f6b656e2d3175
Minted Token: 6d6f380c8699b796e519a48b56e30d61ff2e3c18c4fff3b15dfd2635c59778cb
Mint token, Locking Script: 210211b8b66bda6bcf8538bc847dda0989530ceffc1e2e2089852006f510d58ec411ac0e6578616d706c65746f6b656e2d3275
Minted Token: fc40ccbc36c7e90e75246a41bead99264a30b11ad2c53aba199f19a9a8318aef

=== STEP ===
Broadcast all mints in a single batch using sendWith is performing: all mints are now broadcasted in a single batch using sendWith
--------------------------------------------------
```

Note: The optional redeem flow is present but commented out in main. You can enable it if your environment supports SignAction. It will perform a sequence of NoSend redeems and then send them all with SendWith.

## Integration Steps

1. Ensure your wallet has spendable funds and is configured.
2. Build your first transaction with Options.NoSend=true.
3. Capture NoSendChange from the response and pass it as Options.NoSendChange to the next CreateAction call.
4. Repeat until you’ve staged all related transactions.
5. Call CreateAction once with Options.SendWith containing the list of staged txids to broadcast in one batch.
6. If any transaction requires signatures after construction (e.g., redeem), complete it with SignAction before the batch send.

## Additional Resources

- [NoSend + SendWith Example Code](./no_send_send_with.go)
- [Create Data Transaction](../create_data_tx/create_data_tx.md)
- [List Outputs](../list_outputs/list_outputs.md)
- [List Actions](../list_actions/list_actions.md)


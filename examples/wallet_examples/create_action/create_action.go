package main

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

var (
	// DefaultRecipientAddress is the address to send satoshis to (P2PKH address)
	DefaultRecipientAddress = "1A6ut1tWnfg5mAD8s1drDLM6gNsLNGvgWq"

	// DefaultSatoshisToSend is the amount to send to the recipient
	DefaultSatoshisToSend = uint64(100)

	// DefaultOutputDescription describes the purpose of this output
	DefaultOutputDescription = "Payment to recipient"

	// DefaultTransactionDescription describes the purpose of this transaction
	DefaultTransactionDescription = "Create action example transaction"

	// DefaultOriginator specifies the originator domain or FQDN used to identify the source of the action request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	DefaultOriginator = "example.com"

)

// createLockingScript creates a P2PKH locking script from the recipient address
func createLockingScript(address string) ([]byte, error) {
	addr, err := script.NewAddressFromString(address)
	if err != nil {
		return nil, fmt.Errorf("failed to parse address: %w", err)
	}

	lockingScript, err := p2pkh.Lock(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create P2PKH script: %w", err)
	}

	return lockingScript.Bytes(), nil
}

// createActionArgs creates the arguments needed for the CreateAction
func createActionArgs() (sdk.CreateActionArgs, error) {
	lockingScript, err := createLockingScript(DefaultRecipientAddress)
	if err != nil {
		return sdk.CreateActionArgs{}, fmt.Errorf("failed to create locking script: %w", err)
	}

	signAndProcess := true
	return sdk.CreateActionArgs{
		Description: DefaultTransactionDescription,
		Outputs: []sdk.CreateActionOutput{
			{
				LockingScript:     lockingScript,
				Satoshis:          DefaultSatoshisToSend,
				OutputDescription: DefaultOutputDescription,
				Tags:              []string{"payment", "example"},
			},
		},
		Labels: []string{"create_action_example"},
		Options: &sdk.CreateActionOptions{
			SignAndProcess: &signAndProcess,
		},
	}, nil
}

// This example demonstrates how to create and send a Bitcoin transaction using Alice's wallet.
// The wallet automatically selects UTXOs, creates change outputs, calculates fees, and broadcasts the transaction.
func main() {
	show.ProcessStart("Create Action")
	ctx := context.Background()

	show.Step("Alice", "Creating wallet and setting up environment")
	alice := example_setup.CreateAlice()

	aliceWallet, cleanup, err := alice.CreateWallet(ctx)
	defer cleanup()
	if err != nil {
		panic(fmt.Errorf("failed to create Alice's wallet: %w", err))
	}

	show.Info("Recipient address", DefaultRecipientAddress)

	createArgs, err := createActionArgs()
	if err != nil {
		panic(err)
	}

	show.Step("Alice", fmt.Sprintf("Creating transaction to send %d satoshis", DefaultSatoshisToSend))
	show.Info("Transaction description", DefaultTransactionDescription)
	show.Info("Output description", DefaultOutputDescription)

	result, err := aliceWallet.CreateAction(ctx, createArgs, DefaultOriginator)
	if err != nil {
		panic(fmt.Errorf("failed to create action: %w", err))
	}

	show.WalletSuccess("CreateAction", createArgs, *result)

	if result.Txid.String() != "" {
		show.Transaction(result.Txid.String())
		show.Info("Status", "Transaction successfully created and broadcast")

		if len(result.SendWithResults) > 0 {
			show.Info("Broadcast status", result.SendWithResults[0].Status)
		}
	}

	show.Success("Transaction created and sent successfully")
	show.ProcessComplete("Create Action")
}

/* Output:
🚀 STARTING: Create Action
============================================================

=== STEP ===
Alice is performing: Creating wallet and setting up environment
--------------------------------------------------
CreateWallet: 03aeac4f9aa44ff0a8e54832415cc810d1db8367ccb33febf60cb2fa4f82b5b5c4
Recipient address: 1A6ut1tWnfg5mAD8s1drDLM6gNsLNGvgWq

=== STEP ===
Alice is performing: Creating transaction to send 100 satoshis
--------------------------------------------------
Transaction description: Create action example transaction
Output description: Payment to recipient

 WALLET CALL: CreateAction
Args: {Description:Create action example transaction InputBEEF:[] Inputs:[] Outputs:[{LockingScript:[118 169 20 99 215 90 127 139 69 130 10 199 153 87 39 106 150 29 236 194 12 85 39 136 172] Satoshis:100 OutputDescription:Payment to recipient Basket: CustomInstructions: Tags:[payment example]}] LockTime:<nil> Version:<nil> Labels:[create_action_example] Options:0xc000338080}
✅ Result: {Txid:9cfe180e8ecc7dc3aded5f2322f1c5a6e6de34a3472a21e2e9a5411fc693e0e3 Tx:[1 1 1 1 227 224 147 198 31 65 165 233 226 33 42 71 163 52 222 230 166 197 241 34 35 95 237 173 195 125 204 142 14 24 254 156 2 0 190 239 0 1 0 1 0 0 0 1 180 150 121 103 34 105 49 224 41 66 255 222 63 77 110 6 241 172 120 153 246 107 205 169 0 18 82 75 119 34 17 168 0 0 0 0 0 255 255 255 255 32 116 16 0 0 0 0 0 0 25 118 169 20 238 145 4 48 122 174 211 83 169 153 199 182 242 230 188 37 83 79 252 195 136 172 178 8 0 0 0 0 0 0 25 118 169 20 116 201 141 215 243 172 210 189 58 84 129 198 124 47 52 57 26 39 41 217 136 172 169 15 0 0 0 0 0 0 25 118 169 20 80 211 0 20 228 169 221 130 24 248 27 95 221 237 249 151 24 179 96 65 136 172 210 8 0 0 0 0 0 0 25 118 169 20 114 187 215 11 93 247 171 63 114 248 25 213 22 81 250 6 42 147 36 170 136 172 212 14 0 0 0 0 0 0 25 118 169 20 122 75 211 17 183 200 143 8 229 59 246 8 162 208 49 135 97 226 68 207 136 172 95 10 0 0 0 0 0 0 25 118 169 20 204 97 46 202 51 221 15 28 197 251 60 175 8 33 44 45 0 42 181 149 136 172 250 8 0 0 0 0 0 0 25 118 169 20 139 220 61 120 137 100 245 67 254 103 50 228 162 43 208 214 229 81 187 3 136 172 40 9 0 0 0 0 0 0 25 118 169 20 158 54 135 215 248 49 164 69 187 61 17 138 55 77 235 93 164 145 62 157 136 172 28 11 0 0 0 0 0 0 25 118 169 20 196 177 217 241 233 220 125 188 223 86 167 29 149 93 124 247 5 186 143 170 136 172 113 10 0 0 0 0 0 0 25 118 169 20 113 123 205 14 193 109 89 71 251 68 26 193 156 93 217 253 13 183 254 29 136 172 166 12 0 0 0 0 0 0 25 118 169 20 37 129 14 115 73 234 183 132 74 157 148 52 177 9 20 203 24 248 82 166 136 172 125 9 0 0 0 0 0 0 25 118 169 20 35 161 125 148 53 217 14 47 5 68 157 181 9 161 18 145 225 89 235 249 136 172 44 16 0 0 0 0 0 0 25 118 169 20 92 57 219 35 237 95 233 128 10 69 149 1 248 29 75 17 255 35 9 252 136 172 0 13 0 0 0 0 0 0 25 118 169 20 87 220 58 128 217 72 88 157 36 32 101 144 79 22 117 46 96 235 81 252 136 172 87 15 0 0 0 0 0 0 25 118 169 20 114 132 80 243 70 225 255 55 148 205 248 41 155 62 52 124 216 102 125 32 136 172 24 11 0 0 0 0 0 0 25 118 169 20 151 168 22 6 160 217 245 110 161 175 251 140 228 115 156 138 14 200 242 84 136 172 204 14 0 0 0 0 0 0 25 118 169 20 84 61 29 111 149 106 213 213 108 71 17 151 247 150 240 44 2 135 13 12 136 172 14 14 0 0 0 0 0 0 25 118 169 20 81 81 80 155 250 244 79 138 103 132 4 126 237 143 242 235 54 234 133 123 136 172 2 10 0 0 0 0 0 0 25 118 169 20 157 246 34 61 161 149 218 201 159 142 138 166 34 26 165 9 173 115 63 54 136 172 194 14 0 0 0 0 0 0 25 118 169 20 86 126 44 221 208 61 3 202 201 157 122 119 1 229 251 97 212 43 60 194 136 172 128 12 0 0 0 0 0 0 25 118 169 20 228 203 202 96 138 116 253 102 0 129 206 92 148 54 34 150 210 51 175 59 136 172 84 16 0 0 0 0 0 0 25 118 169 20 0 167 148 64 46 173 185 127 213 167 185 180 7 215 228 142 212 201 246 191 136 172 38 12 0 0 0 0 0 0 25 118 169 20 208 99 180 187 113 104 66 158 51 225 88 118 35 254 43 235 46 85 109 231 136 172 100 0 0 0 0 0 0 0 25 118 169 20 99 215 90 127 139 69 130 10 199 153 87 39 106 150 29 236 194 12 85 39 136 172 254 15 0 0 0 0 0 0 25 118 169 20 235 11 16 32 56 16 170 241 56 176 25 169 84 115 153 73 240 176 174 42 136 172 10 14 0 0 0 0 0 0 25 118 169 20 241 59 200 246 171 133 98 102 233 95 173 204 115 151 46 161 126 201 1 143 136 172 36 15 0 0 0 0 0 0 25 118 169 20 8 76 218 19 26 105 170 224 106 59 3 185 173 152 7 188 91 158 79 142 136 172 207 9 0 0 0 0 0 0 25 118 169 20 117 65 33 5 202 184 170 192 230 101 171 172 155 68 22 188 167 152 23 93 136 172 181 14 0 0 0 0 0 0 25 118 169 20 252 2 154 209 80 26 36 8 69 220 132 109 13 28 104 40 125 214 35 50 136 172 90 10 0 0 0 0 0 0 25 118 169 20 118 9 234 194 254 226 163 164 205 196 221 253 170 231 98 52 39 90 101 5 136 172 100 10 0 0 0 0 0 0 25 118 169 20 86 24 128 13 65 118 102 124 146 6 28 102 242 35 244 37 22 171 83 54 136 172 147 12 0 0 0 0 0 0 25 118 169 20 25 129 1 110 12 125 245 193 80 160 184 137 23 74 137 166 137 164 222 165 136 172 0 0 0 0] NoSendChange:[] SendWithResults:[{Txid:9cfe180e8ecc7dc3aded5f2322f1c5a6e6de34a3472a21e2e9a5411fc693e0e3 Status:sending}] SignableTransaction:<nil>}

🔗 TRANSACTION:
   TxID: 9cfe180e8ecc7dc3aded5f2322f1c5a6e6de34a3472a21e2e9a5411fc693e0e3
Status: Transaction successfully created and broadcast
Broadcast status: sending
✅ SUCCESS: Transaction created and sent successfully
============================================================
🎉 COMPLETED: Create Action
*/

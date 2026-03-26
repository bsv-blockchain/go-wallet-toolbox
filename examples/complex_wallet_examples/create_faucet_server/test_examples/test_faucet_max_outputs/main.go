package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/methods"
)

type faucetReq struct {
	Outputs []methods.FaucetOutput `json:"outputs"`
}

type faucetResp struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Txid    string `json:"txid"`
	BEEFHex string `json:"beef_hex"`
}

func main() {
	// Fill these values before running
	const server = "http://127.0.0.1:8080"               // e.g. "http://127.0.0.1:8080"
	const address = "mhWi6fGQoPZZkqPBZZNHAQfDiS4hC378jT" // destination address to fund
	const amount = 1                                     // satoshis per output
	const numOutputs = 1000                              // number of outputs to create

	if server == "" || address == "" {
		fmt.Println("please set server and address constants in this file")
		os.Exit(1)
	}

	fmt.Println("=== Max Outputs Faucet Funding Request ===")
	fmt.Printf("Server:     %s\n", server)
	fmt.Printf("Address:    %s\n", address)
	fmt.Printf("Amount:     %d satoshi per output\n", amount)
	fmt.Printf("Outputs:    %d\n", numOutputs)
	fmt.Printf("Total:      %d satoshis\n", amount*numOutputs)
	fmt.Println("==========================================")

	// Generate 1000 outputs with the same address and amount
	outputs := make([]methods.FaucetOutput, numOutputs)
	for i := 0; i < numOutputs; i++ {
		outputs[i] = methods.FaucetOutput{
			Address: address,
			Amount:  amount,
		}
	}

	body, err := json.Marshal(faucetReq{
		Outputs: outputs,
	})
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server+"/faucet", bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer func() { _ = resp.Body.Close() }()

	var out faucetResp
	_ = json.NewDecoder(resp.Body).Decode(&out)

	fmt.Println("=== Max Outputs Faucet Funding Response ===")
	fmt.Printf("Status:  %s\n", out.Status)
	if out.Message != "" {
		fmt.Printf("Message: %s\n", out.Message)
	}
	if out.Txid != "" {
		fmt.Printf("TxID:    %s\n", out.Txid)
	}
	if out.BEEFHex != "" {
		fmt.Printf("BEEF:    %s\n", out.BEEFHex)
	}
	fmt.Println("===========================================")
}

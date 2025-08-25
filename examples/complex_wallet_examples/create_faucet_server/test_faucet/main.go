package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type faucetReq struct {
	Address string `json:"address"`
	Amount  uint64 `json:"amount"`
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
	const amount = 1000                                  // satoshis

	if server == "" || address == "" || amount == 0 {
		fmt.Println("please set server, address and amount constants in this file")
		os.Exit(1)
	}

	fmt.Println("=== Faucet Funding Request ===")
	fmt.Printf("Server:  %s\n", server)
	fmt.Printf("Address: %s\n", address)
	fmt.Printf("Amount:  %d satoshis\n", amount)
	fmt.Println("================================")

	body, _ := json.Marshal(faucetReq{Address: address, Amount: amount})
	resp, err := http.Post(server+"/faucet", "application/json", bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var out faucetResp
	_ = json.NewDecoder(resp.Body).Decode(&out)

	fmt.Println("=== Faucet Funding Response ===")
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
	fmt.Println("================================")
}

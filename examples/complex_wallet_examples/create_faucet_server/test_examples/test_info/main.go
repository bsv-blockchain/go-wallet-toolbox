package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type addressResp struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Address string `json:"address"`
	Balance uint64 `json:"balance"`
	Network string `json:"network"`
}

func main() {
	// Fill this value before running
	const server = "http://127.0.0.1:8080" // e.g. "http://127.0.0.1:8080"
	if server == "" {
		fmt.Println("please set server constant in this file")
		os.Exit(1)
	}

	resp, err := http.Get(server + "/info") //nolint:noctx // example script, context not needed
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close() //nolint:errcheck // body close error is not actionable in example code

	var out addressResp
	_ = json.NewDecoder(resp.Body).Decode(&out)

	fmt.Println("=== Faucet Address Response ===")
	fmt.Printf("Status:  %s\n", out.Status)
	fmt.Printf("Address: %s\n", out.Address)
	fmt.Printf("Balance: %d satoshis\n", out.Balance)
	fmt.Printf("Network: %s\n", out.Network)
	if out.Message != "" {
		fmt.Printf("Message: %s\n", out.Message)
	}
	fmt.Println("================================")
}

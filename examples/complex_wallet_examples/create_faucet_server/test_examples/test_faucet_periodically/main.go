package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

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
	const amount = 10                                    // satoshis per call
	const intervalSeconds = 5                            // interval between faucet calls in seconds

	if server == "" || address == "" || amount == 0 {
		fmt.Println("please set server, address and amount constants in this file")
		os.Exit(1)
	}

	fmt.Println("=== Periodic Faucet Funding ===")
	fmt.Printf("Server:   %s\n", server)
	fmt.Printf("Address:  %s (Amount: %d satoshis)\n", address, amount)
	fmt.Printf("Interval: %d seconds\n", intervalSeconds)
	fmt.Println("Press 'x' and Enter to exit")
	fmt.Println("================================")

	// Create a channel to listen for user input
	exitChan := make(chan bool)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if strings.ToLower(strings.TrimSpace(scanner.Text())) == "x" {
				exitChan <- true
				return
			}
		}
	}()

	// Start periodic faucet calls
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	counter := 0
	for {
		select {
		case <-ticker.C:
			counter++
			fmt.Printf("\n--- Faucet Call #%d ---\n", counter)
			fmt.Printf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))

			body, _ := json.Marshal(faucetReq{ //nolint:errchkjson // error not possible for this well-typed struct
				Outputs: []methods.FaucetOutput{
					{Address: address, Amount: amount},
				},
			})
			resp, err := http.Post(server+"/faucet", "application/json", bytes.NewReader(body)) //nolint:noctx // example script, context not needed
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			defer resp.Body.Close() //nolint:errcheck // body close error is not actionable in example code

			var out faucetResp
			_ = json.NewDecoder(resp.Body).Decode(&out)

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
			fmt.Println("------------------------")

		case <-exitChan:
			fmt.Println("\nExiting periodic faucet calls...")
			return
		}
	}
}

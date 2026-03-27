package main

import (
	"bytes"
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
	const server = "http://127.0.0.1:8080" // e.g. "http://127.0.0.1:8080"

	// Multiple output addresses and amounts
	const address1 = "mhWi6fGQoPZZkqPBZZNHAQfDiS4hC378jT" // first destination address
	const address2 = "mhWi6fGQoPZZkqPBZZNHAQfDiS4hC378jT" // second destination address
	const address3 = "mhWi6fGQoPZZkqPBZZNHAQfDiS4hC378jT" // third destination address
	const address4 = "mhWi6fGQoPZZkqPBZZNHAQfDiS4hC378jT" // fourth destination address
	const address5 = "mhWi6fGQoPZZkqPBZZNHAQfDiS4hC378jT" // fifth destination address

	const amount1 = 1 // satoshis for first address
	const amount2 = 2 // satoshis for second address
	const amount3 = 3 // satoshis for third address
	const amount4 = 4 // satoshis for fourth address
	const amount5 = 5 // satoshis for fifth address

	if server == "" {
		fmt.Println("please set server constant in this file")
		os.Exit(1)
	}

	fmt.Println("=== Multi-Output Faucet Funding Request ===")
	fmt.Printf("Server:   %s\n", server)
	fmt.Printf("Address1: %s (Amount: %d satoshis)\n", address1, amount1)
	fmt.Printf("Address2: %s (Amount: %d satoshis)\n", address2, amount2)
	fmt.Printf("Address3: %s (Amount: %d satoshis)\n", address3, amount3)
	fmt.Printf("Address4: %s (Amount: %d satoshis)\n", address4, amount4)
	fmt.Printf("Address5: %s (Amount: %d satoshis)\n", address5, amount5)
	fmt.Printf("Total:    %d satoshis\n", amount1+amount2+amount3+amount4+amount5)
	fmt.Println("===========================================")

	body, _ := json.Marshal(faucetReq{ //nolint:errchkjson // error not possible for this well-typed struct
		Outputs: []methods.FaucetOutput{
			{Address: address1, Amount: amount1},
			{Address: address2, Amount: amount2},
			{Address: address3, Amount: amount3},
			{Address: address4, Amount: amount4},
			{Address: address5, Amount: amount5},
		},
	})
	resp, err := http.Post(server+"/faucet", "application/json", bytes.NewReader(body)) //nolint:noctx // example script, context not needed
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close() //nolint:errcheck // body close error is not actionable in example code

	var out faucetResp
	_ = json.NewDecoder(resp.Body).Decode(&out)

	fmt.Println("=== Multi-Output Faucet Funding Response ===")
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
	fmt.Println("=============================================")
}

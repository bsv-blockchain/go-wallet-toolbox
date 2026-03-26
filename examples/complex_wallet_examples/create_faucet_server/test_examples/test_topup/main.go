package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type topupReq struct {
	Outpoint string `json:"outpoint"` // Format: "txid:outputIndex"
}

type topupResp struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func main() {
	// Fill these values before running
	const server = "http://127.0.0.1:8080"                                                // e.g. "http://127.0.0.1:8080"
	const outpoint = "bb524b2f3880bd987b7739ed1bad48d6d801a42758777cd8f6425c442d1a9fb1:0" // Format: "txid:outputIndex"

	if server == "" || outpoint == "" {
		fmt.Println("please set server and outpoint constants in this file")
		os.Exit(1)
	}

	fmt.Println("=== Topup Internalization Request ===")
	fmt.Printf("Server:   %s\n", server)
	fmt.Printf("Outpoint: %s\n", outpoint)
	fmt.Println("=====================================")

	body, _ := json.Marshal(topupReq{Outpoint: outpoint})                              //nolint:errchkjson // error not possible for this well-typed struct
	resp, err := http.Post(server+"/topup", "application/json", bytes.NewReader(body)) //nolint:noctx // example script, context not needed
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close() //nolint:errcheck // body close error is not actionable in example code

	var out topupResp
	_ = json.NewDecoder(resp.Body).Decode(&out)

	fmt.Println("=== Topup Internalization Response ===")
	fmt.Printf("Status:  %s\n", out.Status)
	if out.Message != "" {
		fmt.Printf("Message: %s\n", out.Message)
	}
	fmt.Println("======================================")
}

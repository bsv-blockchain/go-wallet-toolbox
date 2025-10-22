package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks"
)

const (
	// NOTE: You can spin up your own local server using:
	// - Node.js example server https://github.com/bsv-blockchain/chaintracks-server
	// - Golang Chaintracks server
	chaintracksURL = "http://localhost:3011/"
)

func main() {
	// Set to LevelDebug to see http request logs
	// slog.SetLogLoggerLevel(slog.LevelDebug)

	chaintr := chaintracks.NewClient(slog.Default(), defs.NetworkMainnet, chaintracksURL)

	getInfo(chaintr)
	getFiatExchangeRates(chaintr)
	height := getPresentHeight(chaintr)
	tipHash := findChainTipHashHex(chaintr)
	findChainTipHeaderHex(chaintr)
	tipHeader := findHeaderHexForHeight(chaintr, height-1)
	findHeaderHexForBlockHash(chaintr, tipHash)

	const numberOfHeadersToGet = 5
	getHeaders(chaintr, height-numberOfHeadersToGet, numberOfHeadersToGet)

	// For example purposes, re-add the tip header - in practice, you'd add new headers only
	addHeaderHex(chaintr, tipHeader)
}

func getInfo(chaintr *chaintracks.Client) {
	info, err := chaintr.GetInfo(context.Background())
	if err != nil {
		panic("failed to get Chaintracks info: " + err.Error())
	}

	show.Info("Chaintracks Info", info)
}

func getPresentHeight(chaintr *chaintracks.Client) uint32 {
	height, err := chaintr.GetPresentHeight(context.Background())
	if err != nil {
		panic("failed to get Chaintracks present height: " + err.Error())
	}

	show.Info("Chaintracks Present Height", height)

	return height
}

func findChainTipHashHex(chaintr *chaintracks.Client) string {
	hashHex, err := chaintr.FindChainTipHashHex(context.Background())
	if err != nil {
		panic("failed to get Chaintracks chain tip hash: " + err.Error())
	}

	show.Info("Chaintracks Chain Tip Hash", hashHex)

	return hashHex
}

func findChainTipHeaderHex(chaintr *chaintracks.Client) {
	header, err := chaintr.FindChainTipHeaderHex(context.Background())
	if err != nil {
		panic("failed to get Chaintracks chain tip header: " + err.Error())
	}

	show.Info("Chaintracks Chain Tip Header", header)
}

func findHeaderHexForHeight(chaintr *chaintracks.Client, height uint32) *chaintracks.BlockHeader {
	header, err := chaintr.FindHeaderHexForHeight(context.Background(), height)
	if err != nil {
		panic("failed to get Chaintracks header for height: " + err.Error())
	}

	show.Info(fmt.Sprintf("Chaintracks Header for Height: %d", height), header)
	return header
}

func findHeaderHexForBlockHash(chaintr *chaintracks.Client, hash string) {
	header, err := chaintr.FindHeaderHexForBlockHash(context.Background(), hash)
	if err != nil {
		panic("failed to get Chaintracks header for block hash: " + err.Error())
	}

	show.Info(fmt.Sprintf("Chaintracks Header for Block Hash: %s", hash), header)
}

func getHeaders(chaintr *chaintracks.Client, height uint32, count uint32) {
	hashedHeaders, err := chaintr.GetHeaders(context.Background(), height, count)
	if err != nil {
		panic("failed to get Chaintracks headers: " + err.Error())
	}

	baseHeaders, err := hashedHeaders.ToBaseBlockHeaders()
	if err != nil {
		panic("failed to convert hashed headers to base headers: " + err.Error())
	}

	show.Info(fmt.Sprintf("Got %d Chaintracks Base Headers", len(baseHeaders)), baseHeaders)
	for i, header := range baseHeaders {
		show.Info(fmt.Sprintf("Header index: %d", i), *header)
	}
}

func getFiatExchangeRates(chaintr *chaintracks.Client) {
	rates, err := chaintr.GetFiatExchangeRates(context.Background())
	if err != nil {
		panic("failed to get Chaintracks fiat exchange rates: " + err.Error())
	}

	show.Info("Chaintracks Fiat Exchange Rates", rates)
}

func addHeaderHex(chaintr *chaintracks.Client, header *chaintracks.BlockHeader) {
	err := chaintr.AddHeader(context.Background(), header.BaseBlockHeader)
	if err != nil {
		panic("failed to add Chaintracks header: " + err.Error())
	}

	show.Info("Successfully added Chaintracks Header", header)
}

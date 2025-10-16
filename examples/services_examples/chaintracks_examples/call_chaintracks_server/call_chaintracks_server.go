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
	height := getPresentHeight(chaintr)
	findChainTipHashHex(chaintr)
	findChainTipHeaderHex(chaintr)
	findHeaderHexForHeight(chaintr, height-1)
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

func findChainTipHashHex(chaintr *chaintracks.Client) {
	hashHex, err := chaintr.FindChainTipHashHex(context.Background())
	if err != nil {
		panic("failed to get Chaintracks chain tip hash: " + err.Error())
	}

	show.Info("Chaintracks Chain Tip Hash", hashHex)
}

func findChainTipHeaderHex(chaintr *chaintracks.Client) {
	header, err := chaintr.FindChainTipHeaderHex(context.Background())
	if err != nil {
		panic("failed to get Chaintracks chain tip header: " + err.Error())
	}

	show.Info("Chaintracks Chain Tip Header", header)
}

func findHeaderHexForHeight(chaintr *chaintracks.Client, height uint32) {
	header, err := chaintr.FindHeaderHexForHeight(context.Background(), height)
	if err != nil {
		panic("failed to get Chaintracks header for height: " + err.Error())
	}

	show.Info(fmt.Sprintf("Chaintracks Header for Height: %d", height), header)
}

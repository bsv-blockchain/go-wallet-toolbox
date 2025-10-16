package main

import (
	"context"
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
	getPresentHeight(chaintr)
}

func getInfo(chaintr *chaintracks.Client) {
	info, err := chaintr.GetInfo(context.Background())
	if err != nil {
		panic("failed to get Chaintracks info: " + err.Error())
	}

	show.Info("Chaintracks Info", info)
}

func getPresentHeight(chaintr *chaintracks.Client) {
	height, err := chaintr.GetPresentHeight(context.Background())
	if err != nil {
		panic("failed to get Chaintracks present height: " + err.Error())
	}

	show.Info("Chaintracks Present Height", height)
}

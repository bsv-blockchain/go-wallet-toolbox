package main

import (
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks"
)

func main() {
	config := defs.DefaultChaintracksServerConfig() // TODO: Allow loading from file/env
	server, err := chaintracks.NewServer(slog.Default(), config)
	if err != nil {
		panic(err)
	}

	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}

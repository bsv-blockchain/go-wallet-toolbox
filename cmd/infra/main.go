package main

import (
	"context"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/infra"
)

func main() {
	server, err := infra.NewServer(
		context.Background(),
		infra.WithConfigFile("infra-config.yaml"),
	)
	if err != nil {
		panic(err)
	}

	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

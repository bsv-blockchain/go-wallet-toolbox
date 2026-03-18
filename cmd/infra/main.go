package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
)

func main() {
	server, err := infra.NewServer(
		context.Background(),
		infra.WithConfigFile("infra-config.yaml"),
	)
	if err != nil {
		panic(err)
	}

	go func() {
		if err = server.ListenAndServe(context.Background()); err != nil {
			panic(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	server.Cleanup()
}

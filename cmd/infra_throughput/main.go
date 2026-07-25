package main

import (
	"context"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // bound to a private port; throughput demo profiling
	"os"
	"os/signal"
	"syscall"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
)

func main() {
	// Profiling endpoint for throughput investigations (demo binary only).
	// Reachable from the host via a compose port mapping when needed.
	go func() {
		_ = http.ListenAndServe(":6060", nil) //nolint:gosec // demo profiling listener
	}()

	server, err := infra.NewServer(
		context.Background(),
		infra.WithConfigFile("infra-config-throughput.yaml"),
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

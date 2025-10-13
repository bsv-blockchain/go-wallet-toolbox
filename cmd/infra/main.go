package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/telemetry"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, tracer, err := telemetry.Init(ctx, telemetry.Config{
		ServiceName: "wallet-toolbox",
		PrettyPrint: false,
	})
	if err != nil {
		panic(fmt.Errorf("failed to initialize telemetry: %w", err))
	}
	defer func() { _ = shutdown(ctx) }()

	ctx, span := tracer.Start(ctx, "main")
	defer span.End()

	server, err := infra.NewServer(
		ctx,
		infra.WithConfigFile("infra-config.yaml"),
		infra.WithTracer(tracer),
	)
	if err != nil {
		panic(fmt.Errorf("failed to initialize server: %w", err))
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			serverErr <- fmt.Errorf("server error: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Received shutdown signal")
	case err := <-serverErr:
		panic(err)
	}
}

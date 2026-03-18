package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/certifier"
)

func main() {
	server, err := certifier.NewServer(
		context.Background(),
		"certifier-config.yaml",
	)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		log.Fatalf("failed to stop server: %v", err)
	}

	log.Println("Server stopped")
}

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cleanup := setupSlog()
	defer cleanup()

	ctx := context.Background()
	config := fixtures.Defaults()

	manager := internal.NewManager(ctx, &config)

	p := tea.NewProgram(tui.NewSelectNetwork(manager))

	_, err := p.Run()
	if err != nil {
		fmt.Println("Oh no:", err)
		os.Exit(1)
	}

	fmt.Println("Closing the program")

	time.Sleep(2 * time.Second)
	fmt.Println("Exiting program gracefully")
}

func setupSlog() (cleanup func()) {
	startTime := time.Now().Format("2006-01-02_15-04-05")
	var logFilePath = "manual_tests_" + startTime + ".log"
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		panic(fmt.Sprintf("failed to open log file: %v", err))
	}

	handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Info("Slog logger initialized successfully", "log_file", logFilePath)

	cleanup = func() {
		slog.Info("Closing log file.")
		logFile.Close()
	}
	return
}

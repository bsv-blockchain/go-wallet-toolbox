package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/tui"
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
	logFilePath := "manual_tests_" + startTime + ".log"

	cleanupOldLogs("manual_tests_", 3)

	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // path is constructed from a controlled prefix
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
		logFile.Close() //nolint:errcheck,gosec // best-effort cleanup, error not actionable
	}
	return cleanup
}

func cleanupOldLogs(prefix string, maxFiles int) {
	files, err := filepath.Glob(prefix + "*.log")
	if err != nil {
		slog.Error("Failed to list log files", "error", err)
		return
	}

	if len(files) < maxFiles {
		return
	}

	// Extract timestamps from filenames and sort
	type logFileInfo struct {
		path      string
		timestamp string
	}

	var fileInfos []logFileInfo
	for _, file := range files {
		filename := filepath.Base(file)
		if strings.HasPrefix(filename, prefix) {
			timestamp := strings.TrimSuffix(strings.TrimPrefix(filename, prefix), ".log")
			fileInfos = append(fileInfos, logFileInfo{path: file, timestamp: timestamp})
		}
	}

	// Sort by timestamp (oldest first)
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].timestamp < fileInfos[j].timestamp
	})

	// Remove oldest files
	filesToRemove := len(fileInfos) - maxFiles + 1
	for i := 0; i < filesToRemove; i++ {
		err := os.Remove(fileInfos[i].path)
		if err != nil {
			slog.Error("Failed to remove old log file", "file", fileInfos[i].path, "error", err)
		} else {
			slog.Info("Removed old log file", "file", fileInfos[i].path)
		}
	}
}

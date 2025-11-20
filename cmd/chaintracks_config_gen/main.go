package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func main() {
	outputFile := flag.String("output-file", "chaintracks-config.yaml", "Output configuration file path")
	flag.StringVar(outputFile, "o", "chaintracks-config.yaml", "Output configuration file path (shorthand)")
	flag.Parse()

	cfg := defs.DefaultChaintracksServerConfig()

	err := config.ToYAMLFile(cfg, *outputFile)
	if err != nil {
		log.Fatalf("Error writing configuration: %v\n", err)
	}

	fmt.Printf("Chaintracks configuration written to %s\n", *outputFile)
}

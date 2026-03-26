package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
)

func main() {
	genKey := flag.Bool("gen-key", false, "Generate a server private key")
	flag.BoolVar(genKey, "k", false, "Generate a server private key (shorthand)")

	outputFile := flag.String("output-file", "infra-config.yaml", "Output configuration file path")
	flag.StringVar(outputFile, "o", "infra-config.yaml", "Output configuration file path (shorthand)")

	flag.Parse()

	cfg := infra.Defaults()

	if *genKey {
		key, err := generateServerPrivateKey()
		if err != nil {
			log.Fatalf("Error generating private key: %v\n", err)
		}
		cfg.ServerPrivateKey = key
	}

	err := cfg.ToYAMLFile(*outputFile)
	if err != nil {
		log.Fatalf("Error writing configuration: %v\n", err)
	}

	fmt.Printf("Configuration written to %s\n", *outputFile)
}

func generateServerPrivateKey() (string, error) {
	priv, err := ec.NewPrivateKey()
	if err != nil {
		return "", fmt.Errorf("couldn't generate private key: %w", err)
	}
	privBytes := priv.Serialize()
	privHex := hex.EncodeToString(privBytes)
	return privHex, nil
}

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func main() {

	outputFile := flag.String("output-file", "examples/internal/example_setup/examples-config.yaml", "Output configuration file path")
	flag.StringVar(outputFile, "o", "examples/internal/example_setup/examples-config.yaml", "Output configuration file path (shorthand)")

	network := flag.String("network", string(defs.NetworkTestnet), "BSV network (test, main, regtest)")
	flag.StringVar(network, "n", string(defs.NetworkTestnet), "BSV network (shorthand)")

	serverURL := flag.String("server-url", "http://localhost:8100", "Server URL")
	flag.StringVar(serverURL, "s", "http://localhost:8100", "Server URL (shorthand)")

	flag.Parse()

	_, err := defs.ParseBSVNetworkStr(*network)
	if err != nil {
		log.Fatalf("Invalid network '%s': %v", *network, err)
	}

	alice, err := generateUserConfig()
	if err != nil {
		log.Fatalf("Error generating Alice config: %v", err)
	}

	bob, err := generateUserConfig()
	if err != nil {
		log.Fatalf("Error generating Bob config: %v", err)
	}

	cfg := example_setup.SetupConfig{
		Network:   defs.BSVNetwork(*network),
		ServerURL: *serverURL,
		Alice:     alice,
		Bob:       bob,
	}

	err = cfg.Validate()
	if err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	err = cfg.ToYAMLFile(*outputFile)
	if err != nil {
		log.Fatalf("Error writing configuration: %v", err)
	}

	fmt.Printf("Configuration written to %s\n", *outputFile)
}

func generateUserConfig() (example_setup.UserConfig, error) {
	privKey, err := ec.NewPrivateKey()
	if err != nil {
		return example_setup.UserConfig{}, fmt.Errorf("failed to generate private key: %w", err)
	}

	return example_setup.UserConfig{
		IdentityKey: privKey.PubKey().ToDERHex(),
		PrivateKey:  hex.EncodeToString(privKey.Serialize()),
	}, nil
}

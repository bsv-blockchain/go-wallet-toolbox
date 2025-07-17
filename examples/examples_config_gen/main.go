package main

import (
	"encoding/hex"
	"flag"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

func main() {

	var outputFile string
	flag.StringVar(&outputFile, "output-file", "examples/internal/example_setup/examples-config.yaml", "Output configuration file path")
	flag.StringVar(&outputFile, "o", "examples/internal/example_setup/examples-config.yaml", "Output configuration file path (shorthand)")

	var network string
	flag.StringVar(&network, "network", string(defs.NetworkTestnet), "BSV network (test, main)")
	flag.StringVar(&network, "n", string(defs.NetworkTestnet), "BSV network (shorthand)")

	var serverURL string
	flag.StringVar(&serverURL, "server-url", "http://localhost:8100", "Server URL")
	flag.StringVar(&serverURL, "s", "http://localhost:8100", "Server URL (shorthand)")

	flag.Parse()

	_, err := defs.ParseBSVNetworkStr(network)
	if err != nil {
		panic(fmt.Errorf("Invalid network '%s': %v", network, err))
	}

	alice, err := generateUserConfig()
	if err != nil {
		panic(fmt.Errorf("Error generating Alice config: %v", err))
	}

	bob, err := generateUserConfig()
	if err != nil {
		panic(fmt.Errorf("Error generating Bob config: %v", err))
	}

	cfg := example_setup.SetupConfig{
		Network:   defs.BSVNetwork(network),
		ServerURL: serverURL,
		Alice:     alice,
		Bob:       bob,
	}

	err = cfg.Validate()
	if err != nil {
		panic(fmt.Errorf("Configuration validation failed: %v", err))
	}

	err = cfg.ToYAMLFile(outputFile)
	if err != nil {
		panic(fmt.Errorf("Error writing configuration: %v", err))
	}

	show.Info("Configuration written to", outputFile)
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

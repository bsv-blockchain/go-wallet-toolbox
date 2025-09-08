package fixtures

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

type Config struct {
	BSVNetwork       defs.BSVNetwork `mapstructure:"bsv_network"`
	ServerPrivateKey string          `mapstructure:"server_private_key"`
	Alice            UserConfig      `mapstructure:"alice"`
	Bob              UserConfig      `mapstructure:"bob"`
}

func Defaults() Config {
	return Config{
		BSVNetwork:       defs.NetworkTestnet,
		ServerPrivateKey: "8143f5ed6c5b41c3d084d39d49e161d8dde4b50b0685a4e4ac23959d3b8a319b",
		Alice: UserConfig{
			Name:    "Alice",
			PrivKey: "143ab18a84d3b25e1a13cefa90038411e5d2014590a2a4a57263d1593c8dee1c",
		},
		Bob: UserConfig{
			Name:    "Bob",
			PrivKey: "0881208859876fc227d71bfb8b91814462c5164b6fee27e614798f6e85d2547d",
		},
	}
}

func (c *Config) Validate() error {
	var err error
	if c.BSVNetwork, err = defs.ParseBSVNetworkStr(string(c.BSVNetwork)); err != nil {
		return fmt.Errorf("invalid BSV network: %w", err)
	}

	if c.ServerPrivateKey == "" {
		return fmt.Errorf("server_private_key is required")
	}

	if err := c.Alice.Validate(); err != nil {
		return err
	}

	if err := c.Bob.Validate(); err != nil {
		return err
	}

	return nil
}

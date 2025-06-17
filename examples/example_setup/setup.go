package example_setup

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/wallet"
)

type Setup struct {
	environment Environment
	identityKey ec.PublicKey
	privateKey  ec.PrivateKey
}

func (s *Setup) CreateWallet() (*wallet.Wallet, func()) {
	// 1. Create a new wallet from an setup.environment
	// - I would like to have this also as an example for us how to improve wallet.New so that it would accept some config struct.
	return nil, func() {
		// cleanup function
	}
}

type Environment struct {
	BSVNetwork defs.BSVNetwork `mapstructure:"bsv_network"`
}

func CreateAlice() *Setup {
	// 1. Load the Environment config with use of viper,
	// 		a. see how we initialize infra.Config struct
	//      b. make sure to validate it also here
	//      c. I see 2 possible options to structurize config files
	//          1. dedicated config file for each "user" - this seems to be more flexible - because we could allow adopters to specify the file from which to load the config
	//          2. one config file with dedicated sections for each "user" - this seems to be more convenient and consistent because everything is in one place and for example network would be consistent between all users
	// 		d. It would be good to also handle --network flag passed to the command (main()), viper have option to also handle the flags.
	// 2. Initialize Setup fields based on network and config/environment
	// 3. In case of errors, panic
	return nil
}

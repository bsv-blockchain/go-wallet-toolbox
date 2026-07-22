package infra

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/server"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
)

//go:generate go run ../../cmd/infra_config_gen/main.go -o ../../infra-config.example.yaml

// Config is the configuration for the "remote storage server" service (aka "infra")
type Config struct {
	// Name is the human-readable name of this storage server
	Name                  string                     `mapstructure:"name"`
	ServerPrivateKey      string                     `mapstructure:"server_private_key"`
	BSVNetwork            defs.BSVNetwork            `mapstructure:"bsv_network"`
	FeeModel              defs.FeeModel              `mapstructure:"fee_model"`
	DBConfig              defs.Database              `mapstructure:"db"`
	HTTPConfig            HTTPConfig                 `mapstructure:"http"`
	Logging               defs.LogConfig             `mapstructure:"logging"`
	Commission            defs.Commission            `mapstructure:"commission"`
	Services              defs.WalletServices        `mapstructure:"wallet_services"`
	Monitor               defs.Monitor               `mapstructure:"monitor"`
	SynchronizeTxStatuses defs.SynchronizeTxStatuses `mapstructure:"synchronize_tx_statuses"`
	FailAbandoned         defs.FailAbandoned         `mapstructure:"fail_abandoned"`
	TracingConfig         defs.TracingConfig         `mapstructure:"tracing"`
	ChangeBasket          defs.ChangeBasket          `mapstructure:"change_basket"`
	UTXOManagement        defs.UTXOManagement        `mapstructure:"utxo_management"`
	Observability         defs.Observability         `mapstructure:"observability"`
}

// DBConfig is the configuration for the database
type DBConfig struct {
	Engine defs.DBType `mapstructure:"engine"`
}

// HTTPConfig is the configuration for the HTTP server related settings
type HTTPConfig struct {
	Port                uint              `mapstructure:"port"`
	RequestPrice        uint              `mapstructure:"request_price"`
	MaxRequestBodyBytes int64             `mapstructure:"max_request_body_bytes"`
	CORS                server.CORSConfig `mapstructure:"cors"`
}

// Validate validates the HTTP configuration
func (c *HTTPConfig) Validate() error {
	const maxPort = 65535
	if c.Port > maxPort {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.MaxRequestBodyBytes <= 0 {
		return fmt.Errorf("max_request_body_bytes must be greater than 0")
	}
	if err := c.CORS.Validate(); err != nil {
		return fmt.Errorf("invalid CORS config: %w", err)
	}

	_, err := satoshi.From(c.RequestPrice)
	if err != nil {
		return fmt.Errorf("invalid request price in satoshis: %w", err)
	}

	return nil
}

// Defaults returns the default configuration
func Defaults() Config {
	network := defs.NetworkMainnet

	return Config{
		Name:             "go-storage-server",
		ServerPrivateKey: "", // it is not optional, user must provide it

		BSVNetwork: network,
		DBConfig:   defs.DefaultDBConfig(),
		HTTPConfig: HTTPConfig{
			Port:                8100,
			RequestPrice:        0,
			MaxRequestBodyBytes: server.DefaultMaxRequestBodyBytes,
			CORS:                storage.DefaultCORSConfig(),
		},
		FeeModel:              defs.DefaultFeeModel(),
		Logging:               defs.DefaultLogConfig(),
		Commission:            defs.DefaultCommission(),
		Services:              defs.DefaultServicesConfig(network),
		Monitor:               defs.DefaultMonitorConfig(),
		SynchronizeTxStatuses: defs.DefaultSynchronizeTxStatuses(),
		FailAbandoned:         defs.DefaultFailAbandoned(),
		TracingConfig:         defs.DefaultTracingConfig(),
		ChangeBasket:          defs.DefaultChangeBasket(),
		UTXOManagement:        defs.DefaultUTXOManagement(),
		Observability:         defs.DefaultObservability(),
	}
}

// OnPostLoad is called after the configuration is loaded
func (c *Config) OnPostLoad() error {
	var err error
	if c.BSVNetwork, err = defs.ParseBSVNetworkStr(string(c.BSVNetwork)); err != nil {
		return fmt.Errorf("invalid BSV network: %w", err)
	}
	c.Services.Chain = c.BSVNetwork

	// The service defaults baked in by Defaults() are for mainnet (the network is only
	// known after the config file/env are loaded). Re-derive the network-specific service
	// endpoints for the selected network, preserving any operator overrides.
	c.reconcileServiceDefaultsForNetwork()
	return nil
}

// reconcileServiceDefaultsForNetwork swaps service endpoints that still carry the mainnet
// defaults (from Defaults()) over to the defaults for the selected network. Operator
// overrides in the config file/env are preserved: a field is only adjusted when it still
// equals the mainnet default value the operator did not touch. This generalises the former
// testnet-ARC-only swap so that test/ttn/tstn also get coherent Arcade / GorillaPool /
// WhatsOnChain / ChainTracks defaults (and for tstn, the TSTN_ARCADE_URL / TSTN_CHAINTRACKS_URL
// runtime endpoints).
func (c *Config) reconcileServiceDefaultsForNetwork() {
	if c.BSVNetwork == defs.NetworkMainnet {
		return // Defaults() already holds the mainnet service defaults.
	}

	mainDefaults := defs.DefaultServicesConfig(defs.NetworkMainnet)
	target := defs.DefaultServicesConfig(c.BSVNetwork)
	s := &c.Services

	// ARC (TAAL / primary merkle-path source)
	if s.ArcConfig.URL == mainDefaults.ArcConfig.URL {
		s.ArcConfig.URL = target.ArcConfig.URL
	}
	if s.ArcConfig.Token == mainDefaults.ArcConfig.Token {
		s.ArcConfig.Token = target.ArcConfig.Token
	}

	// Arcade (primary broadcaster)
	if s.Arcade.Enabled == mainDefaults.Arcade.Enabled {
		s.Arcade.Enabled = target.Arcade.Enabled
	}
	if s.Arcade.URL == mainDefaults.Arcade.URL {
		s.Arcade.URL = target.Arcade.URL
	}
	if s.Arcade.EventsURL == mainDefaults.Arcade.EventsURL {
		s.Arcade.EventsURL = target.Arcade.EventsURL
	}

	// GorillaPool ARC (mainnet-only broadcast failover)
	if s.ArcGorillaPoolConfig.Enabled == mainDefaults.ArcGorillaPoolConfig.Enabled {
		s.ArcGorillaPoolConfig.Enabled = target.ArcGorillaPoolConfig.Enabled
	}
	if s.ArcGorillaPoolConfig.URL == mainDefaults.ArcGorillaPoolConfig.URL {
		s.ArcGorillaPoolConfig.URL = target.ArcGorillaPoolConfig.URL
	}

	// WhatsOnChain (absent on tstn)
	if s.WhatsOnChain.Enabled == mainDefaults.WhatsOnChain.Enabled {
		s.WhatsOnChain.Enabled = target.WhatsOnChain.Enabled
	}

	// ChainTracks (headers; enabled by default on the Teranode networks)
	if s.ChaintracksClient.Enabled == mainDefaults.ChaintracksClient.Enabled {
		s.ChaintracksClient.Enabled = target.ChaintracksClient.Enabled
	}
	if s.ChaintracksClient.Mode == mainDefaults.ChaintracksClient.Mode {
		s.ChaintracksClient.Mode = target.ChaintracksClient.Mode
	}
	if s.ChaintracksClient.RemoteURL == mainDefaults.ChaintracksClient.RemoteURL {
		s.ChaintracksClient.RemoteURL = target.ChaintracksClient.RemoteURL
	}
}

// Validate validates the whole configuration
func (c *Config) Validate() (err error) {
	if c.ServerPrivateKey == "" {
		return fmt.Errorf("server private key is required")
	}
	if c.BSVNetwork, err = defs.ParseBSVNetworkStr(string(c.BSVNetwork)); err != nil {
		return fmt.Errorf("invalid BSV network: %w", err)
	}

	if err = c.HTTPConfig.Validate(); err != nil {
		return fmt.Errorf("invalid HTTP config: %w", err)
	}

	if err = c.FeeModel.Validate(); err != nil {
		return fmt.Errorf("invalid fee model: %w", err)
	}

	if err = c.DBConfig.Validate(); err != nil {
		return fmt.Errorf("invalid DB config: %w", err)
	}

	if err = c.Logging.Validate(); err != nil {
		return fmt.Errorf("invalid logging config: %w", err)
	}

	if err = c.Commission.Validate(); err != nil {
		return fmt.Errorf("invalid commission config: %w", err)
	}

	if err = c.Services.Validate(); err != nil {
		return fmt.Errorf("invalid services config: %w", err)
	}

	if err = c.Monitor.Validate(); err != nil {
		return fmt.Errorf("invalid monitor config: %w", err)
	}

	if err = c.TracingConfig.Validate(); err != nil {
		return fmt.Errorf("invalid tracing config: %w", err)
	}

	if err = c.UTXOManagement.Validate(c.FeeModel, c.Commission); err != nil {
		return fmt.Errorf("invalid utxo management config: %w", err)
	}

	if err = c.Observability.Validate(); err != nil {
		return fmt.Errorf("invalid observability config: %w", err)
	}
	// Metrics ride the tracing OTLP endpoint; tracing.enabled gates spans only.
	if c.Observability.Metrics.Enabled && c.TracingConfig.DialAddr == "" {
		return fmt.Errorf("observability.metrics.enabled requires tracing.dialAddr to be set")
	}

	return nil
}

// Validate validates the DB configuration
func (c *DBConfig) Validate() (err error) {
	if c.Engine, err = defs.ParseDBTypeStr(string(c.Engine)); err != nil {
		return fmt.Errorf("invalid DB engine: %w", err)
	}

	return nil
}

// ToYAMLFile writes the configuration to a YAML file
func (c *Config) ToYAMLFile(filename string) error {
	err := config.ToYAMLFile(c, filename)
	if err != nil {
		return fmt.Errorf("failed to write config to file: %w", err)
	}
	return nil
}

package defs

import "fmt"

// BabbageBlockHeadersCDN is the base URL for fetching BSV block headers from the Project Babbage CDN.
const BabbageBlockHeadersCDN = "https://cdn.projectbabbage.com/blockheaders"

// LiveIngestorType represents the type of a live blockchain data ingestor as a string identifier.
type LiveIngestorType string

// LiveIngestorTypeWocPoll represents a live ingestor type that polls block headers from WhatsOnChain's public API.
const (
	LiveIngestorTypeWocPoll LiveIngestorType = "woc_poll"
)

// ParseLiveIngestorType parses a string into a LiveIngestorType, allowing case-insensitive matching of known types.
// Returns an error if the input does not match any supported LiveIngestorType value.
func ParseLiveIngestorType(str string) (LiveIngestorType, error) {
	return parseEnumCaseInsensitive(str, LiveIngestorTypeWocPoll)
}

// ChaintracksServiceConfig holds configuration for Chaintracks service, including the BSV network selection.
type ChaintracksServiceConfig struct {
	Chain            BSVNetwork              `mapstructure:"-"`
	LiveIngestors    []LiveIngestorType      `mapstructure:"live_ingestors"`
	CDNBulkIngestors []CDNBulkIngestorConfig `mapstructure:"cdn_bulk_ingestors"`

	// TODO: Specify API key for WoC ingestor
}

// Validate checks if the Chain field in ChaintracksServiceConfig holds a valid BSV network type.
// Returns an error if the Chain value is not supported.
// Intended to ensure correct service configuration before server initialization or handler registration.
func (c *ChaintracksServiceConfig) Validate() error {
	if err := c.Chain.Validate(); err != nil {
		return fmt.Errorf("invalid chain: %w", err)
	}

	if len(c.LiveIngestors) == 0 {
		return fmt.Errorf("at least one live ingestor must be configured")
	}

	var err error
	for i := range c.LiveIngestors {
		if c.LiveIngestors[i], err = ParseLiveIngestorType(string(c.LiveIngestors[i])); err != nil {
			return fmt.Errorf("invalid live ingestor type: %s", c.LiveIngestors[i])
		}
	}

	for i := range c.CDNBulkIngestors {
		if err := c.CDNBulkIngestors[i].Validate(); err != nil {
			return fmt.Errorf("invalid CDN bulk ingestor config at index %d: %w", i, err)
		}
	}

	return nil
}

// DefaultChaintracksServiceConfig returns a ChaintracksServiceConfig set to use the mainnet BSV network by default.
func DefaultChaintracksServiceConfig() ChaintracksServiceConfig {
	return ChaintracksServiceConfig{
		Chain: NetworkMainnet,
		LiveIngestors: []LiveIngestorType{
			LiveIngestorTypeWocPoll,
		},
		CDNBulkIngestors: []CDNBulkIngestorConfig{
			{
				SourceURL: BabbageBlockHeadersCDN,
			},
		},
	}
}

// CDNBulkIngestorConfig holds configuration options for a bulk ingestor that fetches block headers from a CDN source.
type CDNBulkIngestorConfig struct {
	SourceURL string     `mapstructure:"source_url"`
}

// Validate checks if the CDNBulkIngestorConfig is valid by ensuring the SourceURL field is not empty.
// Returns an error if SourceURL is missing.
func (c *CDNBulkIngestorConfig) Validate() error {
	if c.SourceURL == "" {
		return fmt.Errorf("source_url is required for CDN bulk ingestor")
	}
	return nil
}

// ChaintracksServerConfig holds the configuration for the Chaintracks HTTP server and its underlying service settings.
type ChaintracksServerConfig struct {
	ChaintracksServiceConfig
	Port          uint   `mapstructure:"port"`
	RoutingPrefix string `mapstructure:"routing_prefix"`
}

// Validate checks if the ChaintracksServerConfig fields contain valid values and returns an error if any are invalid.
func (c *ChaintracksServerConfig) Validate() error {
	if err := c.ChaintracksServiceConfig.Validate(); err != nil {
		return fmt.Errorf("invalid chaintracks service config: %w", err)
	}

	const maxPort = 65535
	if c.Port == 0 || c.Port > maxPort {
		return fmt.Errorf("invalid port: %d", c.Port)
	}

	return nil
}

// DefaultChaintracksServerConfig returns a ChaintracksServerConfig with default settings for the Chaintracks server.
// Sets port to 3011 and applies the default ChaintracksServiceConfig using the mainnet BSV network.
// Intended to provide a ready-to-use configuration for starting a Chaintracks server instance.
func DefaultChaintracksServerConfig() ChaintracksServerConfig {
	return ChaintracksServerConfig{
		Port:                     3011,
		ChaintracksServiceConfig: DefaultChaintracksServiceConfig(),
	}
}

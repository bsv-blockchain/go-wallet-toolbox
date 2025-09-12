package defs

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ARC is a configuration for ARCService used by the wallet service to communicate with ARC.
type ARC struct {
	Enabled bool `mapstructure:"enabled"`
	// URL is the base URL of the ARC service.
	URL string `mapstructure:"url"`
	// Token is the authentication token for the ARC service.
	Token string `mapstructure:"token"`
	// DeploymentID is the ID of this deployment to be announced to ARC - this is helpful for issue tracking.
	DeploymentID string `mapstructure:"deployment_id"`
	// WaitFor is the transaction status for which ARCService should wait when broadcasting transaction.
	WaitFor string `mapstructure:"wait_for"`
	// CallbackURL is the URL to which ARC will send a callback after processing the transaction.
	CallbackURL string `mapstructure:"callback_url"`
	// CallbackToken is the token used for authentication in the callback URL.
	CallbackToken string `mapstructure:"callback_token"`
}

// Validate checks if the ARC configuration is valid.
func (arc *ARC) Validate() error {
	if !arc.Enabled {
		return nil
	}

	if err := arc.validateCallbackURL(); err != nil {
		return fmt.Errorf("invalid callback URL: %w", err)
	}

	return nil
}

func (arc *ARC) validateCallbackURL() error {
	if arc.CallbackURL == "" {
		return nil
	}

	callbackURLString := arc.CallbackURL

	callbackURL, err := url.Parse(callbackURLString)
	if err != nil {
		return fmt.Errorf("invalid callback URL: %s - %w", callbackURLString, err)
	}

	schema := callbackURL.Scheme
	if schema != "http" && schema != "https" {
		return fmt.Errorf("invalid callback URL: %s - it should start with http:// or https://", callbackURLString)
	}

	hostname := callbackURL.Hostname()

	if arc.isLocalNetworkHost(hostname) {
		return fmt.Errorf("invalid callback host: %s - must be a valid external URL - not a localhost", hostname)
	}

	return nil
}

func (arc *ARC) isLocalNetworkHost(hostname string) bool {
	if strings.Contains(hostname, "localhost") {
		return true
	}

	ip := net.ParseIP(hostname)
	if ip != nil {
		_, private10, _ := net.ParseCIDR("10.0.0.0/8")
		_, private172, _ := net.ParseCIDR("172.16.0.0/12")
		_, private192, _ := net.ParseCIDR("192.168.0.0/16")
		_, loopback, _ := net.ParseCIDR("127.0.0.0/8")
		_, linkLocal, _ := net.ParseCIDR("169.254.0.0/16")

		return private10.Contains(ip) || private172.Contains(ip) || private192.Contains(ip) || loopback.Contains(ip) || linkLocal.Contains(ip)
	}

	return false
}

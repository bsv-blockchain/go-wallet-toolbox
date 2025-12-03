package wallet_settings_manager

import (
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/go-softwarelab/common/pkg/to"
)

// Default certifiers for mainnet
var defaultCertifiers = []Certifier{
	{
		Name:        "Metanet Trust Services",
		Description: "Registry for protocols, baskets, and certificate types",
		IconURL:     to.Ptr("https://bsvblockchain.org/favicon.ico"),
		IdentityKey: "03daf815fe38f83da0ad83b5bedc520aa488aef5cbc93a93c67a7fe60406cbffe8",
		Trust:       4,
	},
	{
		Name:        "SocialCert",
		Description: "Certifies social media handles, phone numbers and emails",
		IconURL:     to.Ptr("https://socialcert.net/favicon.ico"),
		IdentityKey: "02cf6cdf466951d8dfc9e7c9367511d0007ed6fba35ed42d425cc412fd6cfd4a17",
		Trust:       3,
	},
}

// Testnet identity key mappings
var testnetIdentityKeys = map[string]string{
	"Babbage Trust Services": "03d0b36b5c98b000ec9ffed9a2cf005e279244edf6a19cf90545cdebe873162761",
	"IdentiCert":             "036dc48522aba1705afbb43df3c04dbd1da373b6154341a875bceaa2a3e7f21528",
	"SocialCert":             "02cf6cdf466951d8dfc9e7c9367511d0007ed6fba35ed42d425cc412fd6cfd4a17",
}

// TrustSettingsCache is a struct for the single trust settings cache
type TrustSettingsCache struct {
	ExpiresAt     time.Time
	TrustSettings *TrustSettings
}

// TrustSettings contains the trust level and list of trusted certifiers
type TrustSettings struct {
	TrustLevel        int
	TrustedCertifiers []Certifier
}

// Certifier represents a trusted certificate authority
type Certifier struct {
	Name        string
	Description string
	IdentityKey string
	Trust       int
	IconURL     *string
	BaseURL     *string
}

// WalletTheme represents theme settings
type WalletTheme struct {
	Mode string
}

// WalletSettings is the complete settings structure for wallet
type WalletSettings struct {
	TrustSettings  *TrustSettings
	Theme          *WalletTheme
	Currency       *string
	PermissionMode *string
}

// WalletSettingsManager manages wallet settings
type WalletSettingsManager struct {
	settings *WalletSettings
	mu       sync.RWMutex
}

func NewWalletSettingsManager(settings *WalletSettings) *WalletSettingsManager {
	return &WalletSettingsManager{
		settings: settings,
	}
}

// Get returns the current wallet settings. Settings should not be modified when using this method.
func (wsm *WalletSettingsManager) Get() *WalletSettings {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()
	return wsm.settings
}

// Set assigns the current wallet settings to new settings
func (wsm *WalletSettingsManager) Set(settings *WalletSettings) {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()
	wsm.settings = settings
}

func DefaultManager(chain defs.BSVNetwork) *WalletSettingsManager {
	var trustedCertifiers []Certifier

	switch chain {
	case defs.NetworkTestnet:
		trustedCertifiers = GetTestnetDefaultCertifiers()
	case defs.NetworkMainnet:
		trustedCertifiers = GetDefaultCertifiers()
	default:
		trustedCertifiers = GetDefaultCertifiers()
	}

	settings := &WalletSettings{
		TrustSettings: &TrustSettings{
			TrustLevel:        2,
			TrustedCertifiers: trustedCertifiers,
		},
		Theme: &WalletTheme{
			Mode: "dark",
		},
		PermissionMode: to.Ptr("simple"),
	}

	return &WalletSettingsManager{
		settings: settings,
	}
}

// GetTestnetDefaultCertifiers returns testnet settings with mapped identity keys
func GetTestnetDefaultCertifiers() []Certifier {
	certifiers := make([]Certifier, len(defaultCertifiers))

	for i, cert := range defaultCertifiers {
		certifiers[i] = cert
		// Use testnet key if available, otherwise keep the default
		if testnetKey, exists := testnetIdentityKeys[cert.Name]; exists {
			certifiers[i].IdentityKey = testnetKey
		}
	}

	return certifiers
}

// GetDefaultCertifiers returns mainnet settings
func GetDefaultCertifiers() []Certifier {
	certifiers := make([]Certifier, len(defaultCertifiers))
	copy(certifiers, defaultCertifiers)

	return certifiers
}

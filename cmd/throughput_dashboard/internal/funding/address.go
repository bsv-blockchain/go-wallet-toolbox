// Package funding derives the operator deposit address and internalizes top-ups.
package funding

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// Fixed BRC-29 derivation used by faucet examples / loadgen bootstrap.
// Browser WalletClient pays this address; internalize uses AnyoneKey remittance.
const (
	DerivationPrefixB64 = "SfKxPIJNgdI="
	DerivationSuffixB64 = "NaGLC6fMH50="

	// defaultSuggestedSatoshis is 0.001 BSV — enough for demo fan-out rounds.
	defaultSuggestedSatoshis uint64 = 100_000
)

// Info is returned to the dashboard UI for WalletClient payments.
type Info struct {
	Network              string `json:"network"`
	Address              string `json:"address"`
	LockingScriptHex     string `json:"locking_script_hex"`
	DerivationPrefixB64  string `json:"derivation_prefix_b64"`
	DerivationSuffixB64  string `json:"derivation_suffix_b64"`
	SenderIdentityKeyHex string `json:"sender_identity_key_hex"` // AnyoneKey — remittance sender
	OperatorIdentityHex  string `json:"operator_identity_hex"`
	SuggestedSatoshis    uint64 `json:"suggested_satoshis"`
}

// DeriveInfo builds the mainnet/testnet BRC-29 deposit address for the operator.
// Sender is always AnyoneKey; KeyID prefix/suffix are the fixed faucet constants.
func DeriveInfo(priv *ec.PrivateKey, network defs.BSVNetwork, suggested uint64) (Info, error) {
	if priv == nil {
		return Info{}, fmt.Errorf("operator private key is required")
	}
	if err := network.Validate(); err != nil {
		return Info{}, err
	}

	_, anyonePub := sdk.AnyoneKey()
	keyID := brc29.KeyID{
		DerivationPrefix: DerivationPrefixB64,
		DerivationSuffix: DerivationSuffixB64,
	}

	var (
		addr *script.Address
		err  error
	)
	switch network {
	case defs.NetworkTestnet:
		addr, err = brc29.AddressForSelf(anyonePub, keyID, priv, brc29.WithTestNet())
	case defs.NetworkMainnet:
		addr, err = brc29.AddressForSelf(anyonePub, keyID, priv, brc29.WithMainNet())
	default:
		return Info{}, fmt.Errorf("unsupported network: %s", network)
	}
	if err != nil {
		return Info{}, fmt.Errorf("derive brc29 address: %w", err)
	}

	lock, err := p2pkh.Lock(addr)
	if err != nil {
		return Info{}, fmt.Errorf("p2pkh lock: %w", err)
	}

	if suggested == 0 {
		suggested = defaultSuggestedSatoshis
	}

	return Info{
		Network:              string(network),
		Address:              addr.AddressString,
		LockingScriptHex:     hex.EncodeToString(lock.Bytes()),
		DerivationPrefixB64:  DerivationPrefixB64,
		DerivationSuffixB64:  DerivationSuffixB64,
		SenderIdentityKeyHex: anyonePub.ToDERHex(),
		OperatorIdentityHex:  priv.PubKey().ToDERHex(),
		SuggestedSatoshis:    suggested,
	}, nil
}

// AnyonePaymentRemittance returns the wallet-payment remittance for faucet-style deposits.
// Derivation bytes match DerivationPrefixB64 / DerivationSuffixB64; sender is AnyoneKey.
func AnyonePaymentRemittance() (*sdk.Payment, error) {
	prefix, err := base64.StdEncoding.DecodeString(DerivationPrefixB64)
	if err != nil {
		return nil, fmt.Errorf("decode prefix: %w", err)
	}
	suffix, err := base64.StdEncoding.DecodeString(DerivationSuffixB64)
	if err != nil {
		return nil, fmt.Errorf("decode suffix: %w", err)
	}
	_, anyonePub := sdk.AnyoneKey()
	return &sdk.Payment{
		DerivationPrefix:  prefix,
		DerivationSuffix:  suffix,
		SenderIdentityKey: anyonePub,
	}, nil
}

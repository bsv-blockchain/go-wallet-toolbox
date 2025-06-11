package brc29

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk"
	"github.com/bsv-blockchain/go-sdk/script"
)

const forSelf = false

// Address generates a blockchain address according to BRC29 specification.
//
// The sender key can be a private key hex string or a key deriver or ec.PrivateKey.
// The recipient key can be a public key hex or a key deriver or ec.PublicKey.
//
// Additional options allow setting the address network to mainnet or testnet.
func Address[SenderKey CounterpartyPrivateKey, RecipientKey CounterpartyPublicKey](senderPrivateKeySource SenderKey, keyID KeyID, recipientPublicKeySource RecipientKey, opts ...func(*lockOptions)) (*script.Address, error) {
	options := &lockOptions{
		mainNet: true,
	}

	for _, opt := range opts {
		opt(options)
	}

	if err := keyID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid key ID: %w", err)
	}

	senderKeyDeriver, err := toKeyDeriver(senderPrivateKeySource)
	if err != nil {
		return nil, fmt.Errorf("failed to create sender key deriver from %T: %w", senderPrivateKeySource, err)
	}

	recipientIdentityKey, err := toIdentityKey(recipientPublicKeySource)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipient identity key from %T: %w", recipientPublicKeySource, err)
	}

	key, err := senderKeyDeriver.DerivePublicKey(Protocol, keyID.String(), sdk.Counterparty{
		Type:         sdk.CounterpartyTypeOther,
		Counterparty: recipientIdentityKey,
	}, forSelf)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key for BRC29: %w", err)
	}

	address, err := script.NewAddressFromPublicKey(key, options.mainNet)
	if err != nil {
		return nil, fmt.Errorf("failed to create address from public key: %w", err)
	}
	return address, nil
}

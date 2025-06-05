package wallet

import (
	"context"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/validate"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet/internal/wallet_opts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/go-softwarelab/common/pkg/to"
)

var _ sdk.Interface = (*Wallet)(nil)

// Wallet is an implementation of the BRC-100 wallet interface.
type Wallet struct {
	proto      *sdk.ProtoWallet
	storage    wdk.WalletStorage
	keyDeriver *sdk.KeyDeriver
	wallet_opts.Opts
}

// New creates a new Wallet instance with the specified network, key deriver, and storage.
// Returns an error if any required parameter is invalid or missing.
// TODO: add support for opts pattern and handle optional parameters as it is in the Typescript version.
func New[K string | *sdk.KeyDeriver](chain defs.BSVNetwork, key K, activeStorage wdk.WalletStorageProvider) (*Wallet, error) {
	var keyDeriver *sdk.KeyDeriver
	switch k := any(key).(type) {
	case string:
		priv, err := ec.PrivateKeyFromHex(k)
		if err != nil {
			return nil, fmt.Errorf("failed to parse provided private key: %w", err)
		}
		keyDeriver = sdk.NewKeyDeriver(priv)
	case *sdk.KeyDeriver:
		keyDeriver = k
	}

	err := chain.Validate()
	if err != nil {
		return nil, fmt.Errorf("valid chain must be provided: %w", err)
	}

	if keyDeriver == nil {
		return nil, fmt.Errorf("deriver must be provided")
	}

	if activeStorage == nil {
		return nil, fmt.Errorf("active storage must be provided")
	}

	proto, err := sdk.NewProtoWallet(sdk.ProtoWalletArgs{
		Type:       sdk.ProtoWalletArgsTypeKeyDeriver,
		KeyDeriver: keyDeriver,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create proto wallet: %w", err)
	}

	storageManager := storage.NewWalletStorageManager(keyDeriver.IdentityKeyHex(), activeStorage)

	return &Wallet{
		proto:      proto,
		keyDeriver: keyDeriver,
		storage:    storageManager,
		Opts: wallet_opts.Opts{
			IncludeAllSourceTransactions: true,
			AutoKnownTxids:               false,
			TrustSelf:                    to.Ptr(sdk.TrustSelfKnown),
		},
	}, nil
}

// GetPublicKey retrieves a derived or identity public key based on the requested protocol, key ID, counterparty, and other factors.
func (w *Wallet) GetPublicKey(ctx context.Context, args sdk.GetPublicKeyArgs, originator string) (*sdk.GetPublicKeyResult, error) {
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.GetPublicKey(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}
	return res, nil
}

// Encrypt encrypts the provided plaintext data using derived keys, based on the protocol ID, key ID, counterparty, and other factors.
func (w *Wallet) Encrypt(ctx context.Context, args sdk.EncryptArgs, originator string) (*sdk.EncryptResult, error) {
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.Encrypt(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}
	return res, nil
}

// Decrypt decrypts the provided ciphertext using derived keys, based on the protocol ID, key ID, counterparty, and other factors.
func (w *Wallet) Decrypt(ctx context.Context, args sdk.DecryptArgs, originator string) (*sdk.DecryptResult, error) {
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.Decrypt(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return res, nil
}

// CreateHmac creates an HMAC (Hash-based Message Authentication Code) based on the provided data, protocol, key ID, counterparty, and other factors.
func (w *Wallet) CreateHmac(ctx context.Context, args sdk.CreateHmacArgs, originator string) (*sdk.CreateHmacResult, error) {
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.CreateHmac(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to create hmac: %w", err)
	}
	return res, nil
}

// VerifyHmac verifies an HMAC (Hash-based Message Authentication Code) based on the provided data, protocol, key ID, counterparty, and other factors.
func (w *Wallet) VerifyHmac(ctx context.Context, args sdk.VerifyHmacArgs, originator string) (*sdk.VerifyHmacResult, error) {
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.VerifyHmac(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to verify hmac: %w", err)
	}
	return res, nil
}

// CreateSignature creates a digital signature for the provided data or hash using a specific protocol, key, and optionally considering privilege and counterparty.
func (w *Wallet) CreateSignature(ctx context.Context, args sdk.CreateSignatureArgs, originator string) (*sdk.CreateSignatureResult, error) {
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.CreateSignature(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to create signature: %w", err)
	}
	return res, nil
}

// VerifySignature verifies a digital signature for the provided data or hash using a specific protocol, key, and optionally considering privilege and counterparty.
func (w *Wallet) VerifySignature(ctx context.Context, args sdk.VerifySignatureArgs, originator string) (*sdk.VerifySignatureResult, error) {
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.VerifySignature(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}
	return res, nil
}

// CreateAction creates a new Bitcoin transaction based on the provided inputs, outputs, labels, locks, and other options.
func (w *Wallet) CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error) {
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	// TODO: mapping.MapCreateActionArgs should handle known tx ids - needs some merging and validation of BEEF
	wdkArgs := mapping.MapCreateActionArgs(args, w.Opts)

	if err := validate.WalletCreateActionArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid create action args: %w", err)
	}

	return nil, nil
}

// SignAction signs a transaction previously created using CreateAction.
func (w *Wallet) SignAction(ctx context.Context, args sdk.SignActionArgs, originator string) (*sdk.SignActionResult, error) {
	// TODO implement me
	panic("implement me")
}

// AbortAction aborts a transaction that is in progress and has not yet been finalized or sent to the network.
func (w *Wallet) AbortAction(ctx context.Context, args sdk.AbortActionArgs, originator string) (*sdk.AbortActionResult, error) {
	// TODO implement me
	panic("implement me")
}

// ListActions lists all transactions matching the specified labels.
func (w *Wallet) ListActions(ctx context.Context, args sdk.ListActionsArgs, originator string) (*sdk.ListActionsResult, error) {
	// TODO implement me
	panic("implement me")
}

// InternalizeAction submits a transaction to be internalized and optionally labeled, outputs paid to the wallet balance,
// inserted into baskets, and/or tagged.
func (w *Wallet) InternalizeAction(ctx context.Context, args sdk.InternalizeActionArgs, originator string) (*sdk.InternalizeActionResult, error) {
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkArgs := mapping.MapInternalizeActionArgs(args)

	if err := validate.ValidInternalizeActionArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid internalize action args: %w", err)
	}

	result, err := w.storage.InternalizeAction(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to internalize action: %w", err)
	}

	return mapping.MapInternalizeActionResult(result), nil
}

// ListOutputs lists the spendable outputs kept within a specific basket, optionally tagged with specific labels.
func (w *Wallet) ListOutputs(ctx context.Context, args sdk.ListOutputsArgs, originator string) (*sdk.ListOutputsResult, error) {
	// TODO implement me
	panic("implement me")
}

// RelinquishOutput relinquishes an output from a basket, removing it from tracking without spending it.
func (w *Wallet) RelinquishOutput(ctx context.Context, args sdk.RelinquishOutputArgs, originator string) (*sdk.RelinquishOutputResult, error) {
	// TODO implement me
	panic("implement me")
}

// RevealCounterpartyKeyLinkage reveals the key linkage between ourselves and a counterparty, to a particular verifier,
// across all interactions with the counterparty.
func (w *Wallet) RevealCounterpartyKeyLinkage(ctx context.Context, args sdk.RevealCounterpartyKeyLinkageArgs, originator string) (*sdk.RevealCounterpartyKeyLinkageResult, error) {
	// TODO implement me
	panic("implement me")
}

// RevealSpecificKeyLinkage reveals the key linkage between ourselves and a counterparty, to a particular verifier,
// with respect to a specific interaction.
func (w *Wallet) RevealSpecificKeyLinkage(ctx context.Context, args sdk.RevealSpecificKeyLinkageArgs, originator string) (*sdk.RevealSpecificKeyLinkageResult, error) {
	// TODO implement me
	panic("implement me")
}

// AcquireCertificate acquires an identity certificate, whether by acquiring one from the certifier or by directly receiving it.
func (w *Wallet) AcquireCertificate(ctx context.Context, args sdk.AcquireCertificateArgs, originator string) (*sdk.Certificate, error) {
	// TODO implement me
	panic("implement me")
}

// ListCertificates lists identity certificates belonging to the user, filtered by certifier(s) and type(s).
func (w *Wallet) ListCertificates(ctx context.Context, args sdk.ListCertificatesArgs, originator string) (*sdk.ListCertificatesResult, error) {
	// TODO implement me
	panic("implement me")
}

// ProveCertificate proves select fields of an identity certificate, as specified, when requested by a verifier.
func (w *Wallet) ProveCertificate(ctx context.Context, args sdk.ProveCertificateArgs, originator string) (*sdk.ProveCertificateResult, error) {
	// TODO implement me
	panic("implement me")
}

// RelinquishCertificate relinquishes an identity certificate, removing it from the wallet regardless of whether
// the revocation outpoint has become spent.
func (w *Wallet) RelinquishCertificate(ctx context.Context, args sdk.RelinquishCertificateArgs, originator string) (*sdk.RelinquishCertificateResult, error) {
	// TODO implement me
	panic("implement me")
}

// DiscoverByIdentityKey discovers identity certificates, issued to a given identity key by a trusted entity.
func (w *Wallet) DiscoverByIdentityKey(ctx context.Context, args sdk.DiscoverByIdentityKeyArgs, originator string) (*sdk.DiscoverCertificatesResult, error) {
	// TODO implement me
	panic("implement me")
}

// DiscoverByAttributes discovers identity certificates belonging to other users, where the documents contain
// specific attributes, issued by a trusted entity.
func (w *Wallet) DiscoverByAttributes(ctx context.Context, args sdk.DiscoverByAttributesArgs, originator string) (*sdk.DiscoverCertificatesResult, error) {
	// TODO implement me
	panic("implement me")
}

// IsAuthenticated checks the authentication status of the user.
func (w *Wallet) IsAuthenticated(ctx context.Context, args any, originator string) (*sdk.AuthenticatedResult, error) {
	// TODO implement me
	panic("implement me")
}

// WaitForAuthentication continuously waits until the user is authenticated, returning the result once confirmed.
func (w *Wallet) WaitForAuthentication(ctx context.Context, args any, originator string) (*sdk.AuthenticatedResult, error) {
	// TODO implement me
	panic("implement me")
}

// GetHeight retrieves the current height of the blockchain.
func (w *Wallet) GetHeight(ctx context.Context, args any, originator string) (*sdk.GetHeightResult, error) {
	// TODO implement me
	panic("implement me")
}

// GetHeaderForHeight retrieves the block header of a block at a specified height.
func (w *Wallet) GetHeaderForHeight(ctx context.Context, args sdk.GetHeaderArgs, originator string) (*sdk.GetHeaderResult, error) {
	// TODO implement me
	panic("implement me")
}

// GetNetwork retrieves the Bitcoin network the client is using (mainnet or testnet).
func (w *Wallet) GetNetwork(ctx context.Context, args any, originator string) (*sdk.GetNetworkResult, error) {
	// TODO implement me
	panic("implement me")
}

// GetVersion retrieves the current version string of the wallet.
func (w *Wallet) GetVersion(ctx context.Context, args any, originator string) (*sdk.GetVersionResult, error) {
	// TODO implement me
	panic("implement me")
}

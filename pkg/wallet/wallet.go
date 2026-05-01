package wallet

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	clients "github.com/bsv-blockchain/go-sdk/auth/clients/authhttp"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/slogx"
	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"

	walletcerts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/certificates"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/specops"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/actions"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/utils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_opts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_settings_manager"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

var _ sdk.Interface = (*Wallet)(nil)

const discoverCertificatesTTL = 2 * time.Minute

type walletCleanupFunc func()

func (wc walletCleanupFunc) Add(next func()) walletCleanupFunc {
	if wc == nil {
		return next
	}
	return func() {
		wc()
		next()
	}
}

// discoverCertificatesParams holds the parameters for certificate discovery.
type discoverCertificatesParams struct {
	cacheKeyStr string
	query       []byte
}

// cacheEntry is a struct for the map-based overlay cache entries
type cacheEntry struct {
	ExpiresAt time.Time
	Value     []certificates.VerifiableCertificate
}

// cacheKey represents the struct that will be a key in overlayCache
type cacheKey struct {
	Fn          string   `json:"fn"`
	IdentityKey string   `json:"identityKey"`
	Attributes  string   `json:"attributes"`
	Certifiers  []string `json:"certifiers"`
}

// identityQuery is a struct representing query to the lookupResolver
// to fetch certificates by IdentityKey.
type identityQuery struct {
	IdentityKey string   `json:"identityKey"`
	Certifiers  []string `json:"certifiers"`
}

// attributesQuery is a struct representing query to the lookupResolver
// to fetch certificates by Attributes.
type attributesQuery struct {
	Attributes map[string]string `json:"attributes"`
	Certifiers []string          `json:"certifiers"`
}

// Wallet is an implementation of the BRC-100 wallet interface.
type Wallet struct {
	trustSettingsCache      atomic.Pointer[wallet_settings_manager.TrustSettingsCache]
	overlayCache            sync.Map
	settingsManager         *wallet_settings_manager.WalletSettingsManager
	lookupResolver          *lookup.LookupResolver
	proto                   *sdk.ProtoWallet
	storage                 wdk.WalletStorage
	keyDeriver              *sdk.KeyDeriver
	flags                   *wallet_opts.Flags
	services                wdk.Services
	chain                   defs.BSVNetwork
	pendingSignActionsCache pending.SignActionsRepository
	logger                  *slog.Logger
	cleanup                 walletCleanupFunc
	auth                    *clients.AuthFetch
	userParty               string
	randomizer              wdk.Randomizer
}

// WithIncludeAllSourceTransactions - default: `true`
// If true, signableTransactions will include sourceTransaction for each input,
// including those that do not require signature and those that were also contained in the inputBEEF.
func WithIncludeAllSourceTransactions(value bool) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		opts.IncludeAllSourceTransactions = value
	}
}

// WithAutoKnownTxids - default: `false`
// If true, txids that are known to the wallet's party beef do not need to be returned from storage.
func WithAutoKnownTxids(value bool) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		opts.AutoKnownTxids = value
	}
}

// WithAuthHTTPClient configures a custom HTTP client for authenticated requests to certificate authorities.
func WithAuthHTTPClient(client *http.Client) func(*wallet_opts.Opts) {
	return func(o *wallet_opts.Opts) {
		o.Client = client
	}
}

// WithLookupResolver configures a lookup resolver for the wallet.
func WithLookupResolver(lookupResolver *lookup.LookupResolver) func(*wallet_opts.Opts) {
	return func(o *wallet_opts.Opts) {
		o.LookupResolver = lookupResolver
	}
}

// WithTrustSelf - default: `known`
// controls behavior of input BEEF validation.
// If "known", input transactions may omit supporting validity proof data for all TXIDs known to this wallet.
// If "", input BEEFs must be complete and valid.
func WithTrustSelf(value sdk.TrustSelf) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		if value == "" {
			opts.TrustSelf = nil
		} else {
			opts.TrustSelf = &value
		}
	}
}

// WithServices allows to set the wallet services that will be used by the wallet.
func WithServices(services wdk.Services) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		opts.Services = services
	}
}

// WithStorageManager allows passing a pre-built WalletStorageManager instead of
// having the wallet create one internally. This enables the remote-wallet pattern
// where the wallet must exist before the storage client can be created (for auth),
// and storage providers are added to the manager after wallet construction.
func WithStorageManager(mgr wdk.WalletStorage) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		opts.StorageManager = mgr
	}
}

// WithPendingSignActionsRepository sets the SignActionsRepository for wallet options, allowing management of cached actions.
func WithPendingSignActionsRepository(cache pending.SignActionsRepository) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		opts.PendingSignActionsRepo = cache
	}
}

// WithWalletSettingsManager sets the WalletSettingsManager for wallet settings
func WithWalletSettingsManager(settingsManager *wallet_settings_manager.WalletSettingsManager) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		opts.WalletSettingsManager = settingsManager
	}
}

// WithLogger sets the provided slog.Logger to the Logger field in wallet_opts.Opts if the logger is not nil.
func WithLogger(logger *slog.Logger) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		if logger != nil {
			opts.Logger = logger
		}
	}
}

// New creates a new Wallet instance with the specified network, key deriver, and storage.
// Returns an error if any required parameter is invalid or missing.
func New[KeySource PrivateKeySource](chain defs.BSVNetwork, keySource KeySource, activeStorage wdk.WalletStorageProvider, opts ...func(*wallet_opts.Opts)) (*Wallet, error) {
	if activeStorage == nil {
		return nil, fmt.Errorf("active storage must be provided")
	}

	return NewWithStorageFactory(chain, keySource, func() wdk.WalletStorageProvider { return activeStorage }, opts...)
}

// NewWithStorageFactory creates a new Wallet instance with the specified network, key deriver, and storage created with provided storage factory function
func NewWithStorageFactory[KeySource PrivateKeySource, ActiveStorageFactory StorageProviderFactory](chain defs.BSVNetwork, keySource KeySource, activeStorageFactory ActiveStorageFactory, opts ...func(*wallet_opts.Opts)) (*Wallet, error) {
	err := chain.Validate()
	if err != nil {
		return nil, fmt.Errorf("valid chain must be provided: %w", err)
	}

	options := to.OptionsWithDefault(wallet_opts.Opts{
		Flags: wallet_opts.Flags{
			IncludeAllSourceTransactions: true,
			AutoKnownTxids:               false,
			TrustSelf:                    to.Ptr(sdk.TrustSelfKnown),
		},
		Logger:                 slog.Default(),
		Services:               nil,
		PendingSignActionsRepo: nil,
		Client:                 wallet_opts.DefaultClient(),
		WalletSettingsManager:  wallet_settings_manager.DefaultManager(chain),
		LookupResolver: lookup.NewLookupResolver(&lookup.LookupResolver{
			NetworkPreset: mapping.MapToOverlayNetwork(chain),
		}),
	}, opts...)

	keyDeriver, err := toKeyDeriver(keySource)
	if err != nil {
		return nil, fmt.Errorf("failed to create key deriver from key source: %w", err)
	}

	proto, err := sdk.NewProtoWallet(sdk.ProtoWalletArgs{Type: sdk.ProtoWalletArgsTypeKeyDeriver, KeyDeriver: keyDeriver})
	if err != nil {
		return nil, fmt.Errorf("failed to create proto wallet: %w", err)
	}

	userParty := fmt.Sprintf("user %s", keyDeriver.IdentityKey().ToDERHex())
	logger := logging.Child(options.Logger, "wallet").With("walletName", userParty)

	if options.PendingSignActionsRepo == nil {
		options.PendingSignActionsRepo = pending.NewSignActionLocalRepository(logger, pending.DefaultPendingSignActionsTTL)
	}

	w := &Wallet{
		proto:                   proto,
		keyDeriver:              keyDeriver,
		flags:                   &options.Flags,
		services:                options.Services,
		chain:                   chain,
		pendingSignActionsCache: options.PendingSignActionsRepo,
		logger:                  logger,
		userParty:               userParty,
		randomizer:              randomizer.New(),
		settingsManager:         options.WalletSettingsManager,
		lookupResolver:          options.LookupResolver,
	}
	w.auth = clients.New(w, clients.WithHttpClientTransport(options.Client.Transport))

	if options.StorageManager != nil {
		w.storage = options.StorageManager
	} else {
		if activeStorageFactory == nil {
			return nil, fmt.Errorf("active storage factory must be provided when no StorageManager is set")
		}
		activeStorage, storageCleanup, err := toStorageProvider(w, activeStorageFactory)
		if err != nil {
			return nil, fmt.Errorf("failed to create active storage: %w", err)
		}
		w.cleanup = w.cleanup.Add(storageCleanup)

		w.storage = storage.NewWalletStorageManager(keyDeriver.IdentityKey().ToDERHex(), logger, activeStorage)
	}

	return w, nil
}

// GetPublicKey retrieves a derived or identity public key based on the requested protocol, key ID, counterparty, and other factors.
func (w *Wallet) GetPublicKey(ctx context.Context, args sdk.GetPublicKeyArgs, originator string) (*sdk.GetPublicKeyResult, error) {
	w.logger.DebugContext(ctx, "GetPublicKey call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.GetPublicKey(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}
	return res, nil
}

// Encrypt encrypts the provided plaintext data using derived keys, based on the protocol ID, key ID, counterparty, and other factors.
func (w *Wallet) Encrypt(ctx context.Context, args sdk.EncryptArgs, originator string) (*sdk.EncryptResult, error) {
	w.logger.DebugContext(ctx, "Encrypt call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.Encrypt(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}
	return res, nil
}

// Decrypt decrypts the provided ciphertext using derived keys, based on the protocol ID, key ID, counterparty, and other factors.
func (w *Wallet) Decrypt(ctx context.Context, args sdk.DecryptArgs, originator string) (*sdk.DecryptResult, error) {
	w.logger.DebugContext(ctx, "Decrypt call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.Decrypt(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return res, nil
}

// CreateHMAC creates an HMAC (Hash-based Message Authentication Code) based on the provided data, protocol, key ID, counterparty, and other factors.
func (w *Wallet) CreateHMAC(ctx context.Context, args sdk.CreateHMACArgs, originator string) (*sdk.CreateHMACResult, error) {
	w.logger.DebugContext(ctx, "CreateHMAC call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.CreateHMAC(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to create HMAC: %w", err)
	}
	return res, nil
}

// VerifyHMAC verifies an HMAC (Hash-based Message Authentication Code) based on the provided data, protocol, key ID, counterparty, and other factors.
func (w *Wallet) VerifyHMAC(ctx context.Context, args sdk.VerifyHMACArgs, originator string) (*sdk.VerifyHMACResult, error) {
	w.logger.DebugContext(ctx, "VerifyHMAC call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.VerifyHMAC(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to verify HMAC: %w", err)
	}
	return res, nil
}

// CreateSignature creates a digital signature for the provided data or hash using a specific protocol, key, and optionally considering privilege and counterparty.
func (w *Wallet) CreateSignature(ctx context.Context, args sdk.CreateSignatureArgs, originator string) (*sdk.CreateSignatureResult, error) {
	w.logger.DebugContext(ctx, "CreateSignature call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.CreateSignature(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to create signature: %w", err)
	}
	return res, nil
}

// VerifySignature verifies a digital signature for the provided data or hash using a specific protocol, key, and optionally considering privilege and counterparty.
func (w *Wallet) VerifySignature(ctx context.Context, args sdk.VerifySignatureArgs, originator string) (*sdk.VerifySignatureResult, error) {
	w.logger.DebugContext(ctx, "VerifySignature call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.VerifySignature(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}
	return res, nil
}

// CreateAction creates a new Bitcoin transaction based on the provided inputs, outputs, labels, locks, and other options.
func (w *Wallet) CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-CreateAction", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "CreateAction start", slogx.String("originator", originator))
	start := time.Now()
	defer func() { w.logger.DebugContext(ctx, "CreateAction done", slog.Duration("duration", time.Since(start))) }()
	action := &actions.CreateAction{
		KeyDeriver:              w.keyDeriver,
		Storage:                 w.storage,
		WalletOpts:              w.flags,
		PendingSignActionsCache: w.pendingSignActionsCache,
	}

	result, err := action.CreateAction(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("create action failed: %w", err)
	}
	w.logger.DebugContext(ctx, "CreateAction success")
	return result, nil
}

// SignAction signs a transaction previously created using CreateAction.
func (w *Wallet) SignAction(ctx context.Context, args sdk.SignActionArgs, originator string) (*sdk.SignActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-SignAction", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "SignAction start", slogx.String("originator", originator))
	start := time.Now()
	defer func() { w.logger.DebugContext(ctx, "SignAction done", slog.Duration("duration", time.Since(start))) }()
	action := &actions.SignAction{
		Logger:                  w.logger,
		PendingSignActionsCache: w.pendingSignActionsCache,
		Storage:                 w.storage,
	}

	result, err := action.SignAction(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("sign action failed: %w", err)
	}
	w.logger.DebugContext(ctx, "SignAction success")
	return result, nil
}

// AbortAction aborts a transaction that is in progress and has not yet been finalized or sent to the network.
func (w *Wallet) AbortAction(ctx context.Context, args sdk.AbortActionArgs, originator string) (*sdk.AbortActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-AbortAction", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "AbortAction call", slogx.String("originator", originator))
	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkArgs := mapping.MapAbortActionArgs(args)

	if err = validate.ValidAbortActionArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid abort action args: %w", err)
	}

	result, err := w.storage.AbortAction(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to abort action: %w", err)
	}

	return mapping.MapAbortActionResult(result), nil
}

// ListActions lists all transactions matching the specified labels.
func (w *Wallet) ListActions(ctx context.Context, args sdk.ListActionsArgs, originator string) (*sdk.ListActionsResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-ListActions", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "ListActions call", slogx.String("originator", originator))
	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkArgs := mapping.MapListActionsArgs(args)

	if err = validate.ListActionsArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid list actions args: %w", err)
	}

	result, err := w.storage.ListActions(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}

	mappedResult, err := mapping.MapListActionsResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to map list actions result: %w", err)
	}

	return mappedResult, nil
}

// ListFailedActions returns only actions with status 'failed'. If unfail is true, it also requests recovery by adding the 'unfail' label.
func (w *Wallet) ListFailedActions(ctx context.Context, args sdk.ListActionsArgs, unfail bool, originator string) (*sdk.ListActionsResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-ListFailActions", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "ListFailedActions call", slogx.String("originator", originator), slog.Bool("unfail", unfail))
	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	args.Labels = append(args.Labels, specops.ListActionsSpecOpFailedActionsLabel)
	if unfail {
		args.Labels = append(args.Labels, "unfail")
	}

	wdkArgs := mapping.MapListActionsArgs(args)

	if err = validate.ListActionsArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid list actions args: %w", err)
	}

	result, err := w.storage.ListActions(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}

	mappedResult, err := mapping.MapListActionsResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to map list actions result: %w", err)
	}

	return mappedResult, nil
}

// ListTransactions retrieves a list of transactions with their status updates (merkle proofs, block info).
func (w *Wallet) ListTransactions(ctx context.Context, args wdk.ListTransactionsArgs, originator string) (*wdk.ListTransactionsResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-ListTransactions", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "ListTransactions call", slogx.String("originator", originator))
	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	result, err := w.storage.ListTransactions(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	return result, nil
}

// InternalizeAction submits a transaction to be internalized and optionally labeled, outputs paid to the wallet balance,
// inserted into baskets, and/or tagged.
func (w *Wallet) InternalizeAction(ctx context.Context, args sdk.InternalizeActionArgs, originator string) (*sdk.InternalizeActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-InternalizeAction", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "InternalizeAction call", slogx.String("originator", originator))
	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkArgs := mapping.MapInternalizeActionArgs(args)

	if err = validate.WalletInternalizeAction(w.keyDeriver, &wdkArgs); err != nil {
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
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-ListOutputs", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "ListOutputs call", slogx.String("originator", originator))
	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkArgs := mapping.MapListOutputsArgs(args)

	if err = validate.ListOutputsArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid list outputs args: %w", err)
	}

	result, err := w.storage.ListOutputs(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to list outputs: %w", err)
	}

	mappedResult, err := mapping.MapListOutputsResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to map list outputs result: %w", err)
	}

	return mappedResult, nil
}

// RelinquishOutput relinquishes an output from a basket, removing it from tracking without spending it.
func (w *Wallet) RelinquishOutput(ctx context.Context, args sdk.RelinquishOutputArgs, originator string) (*sdk.RelinquishOutputResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-RelinquishOutput", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "RelinquishOutput call", slogx.String("originator", originator))
	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkArgs := mapping.MapRelinquishOutputArgs(args)

	if err = validate.ValidRelinquishOutputArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid relinquish output args: %w", err)
	}

	err = w.storage.RelinquishOutput(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to relinquish output: %w", err)
	}

	return &sdk.RelinquishOutputResult{
		Relinquished: true,
	}, nil
}

// RevealCounterpartyKeyLinkage reveals the key linkage between ourselves and a counterparty, to a particular verifier,
// across all interactions with the counterparty.
func (w *Wallet) RevealCounterpartyKeyLinkage(ctx context.Context, args sdk.RevealCounterpartyKeyLinkageArgs, originator string) (*sdk.RevealCounterpartyKeyLinkageResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-RevealCounterpartyKeyLinkage", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "RevealCounterpartyKeyLinkage call", slogx.String("originator", originator))
	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.RevealCounterpartyKeyLinkage(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to reveal counterparty key linkage: %w", err)
	}
	return res, nil
}

// RevealSpecificKeyLinkage reveals the key linkage between ourselves and a counterparty, to a particular verifier,
// with respect to a specific interaction.
func (w *Wallet) RevealSpecificKeyLinkage(ctx context.Context, args sdk.RevealSpecificKeyLinkageArgs, originator string) (*sdk.RevealSpecificKeyLinkageResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-RevealSpecificKeyLinkage", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "RevealSpecificKeyLinkage call", slogx.String("originator", originator))
	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.RevealSpecificKeyLinkage(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to reveal specific key linkage: %w", err)
	}
	return res, nil
}

// AcquireCertificate acquires an identity certificate, whether by acquiring one from the certifier or by directly receiving it.
func (w *Wallet) AcquireCertificate(ctx context.Context, args sdk.AcquireCertificateArgs, originator string) (*sdk.Certificate, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-AcquireCertificate", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	switch args.AcquisitionProtocol {
	case sdk.AcquisitionProtocolDirect:
		return w.acquireDirectCertificate(ctx, args, originator)
	case sdk.AcquisitionProtocolIssuance:
		return w.acquireIssuanceCertificate(ctx, args, originator)
	default:
		return nil, fmt.Errorf("acquire protocol not recognized, allowed types: [%s, %s]", sdk.AcquisitionProtocolDirect, sdk.AcquisitionProtocolIssuance)
	}
}

func (w *Wallet) acquireIssuanceCertificate(ctx context.Context, args sdk.AcquireCertificateArgs, originator string) (*sdk.Certificate, error) {
	w.logger.DebugContext(ctx, "AcquireCertificateIssuance call", slogx.String("originator", originator))

	// Retrieve authentication info early to fail fast
	auth, err := w.storage.GetAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve auth identity: %w", err)
	}

	// Fetch the identity public key early to fail fast
	key, err := w.GetPublicKey(ctx, sdk.GetPublicKeyArgs{IdentityKey: true}, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}

	// Create nonce for the request
	nonce, err := w.createNonce(ctx, args.Certifier, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to create nonce: %w", err)
	}

	// Prepare the certificate signing request payload with certificate data
	issuanceActionData, err := actions.PrepareIssuanceActionData(ctx, actions.PrepareIssuanceActionDataParams{
		Wallet:      w,
		Args:        args,
		Nonce:       nonce,
		IdentityKey: key.PublicKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare issuance payload: %w", err)
	}

	// Send authenticated request to certifier
	url := fmt.Sprintf("%s/signCertificate", args.CertifierUrl)
	res, err := w.auth.Fetch(ctx, url, &clients.SimplifiedFetchRequestOptions{
		Method:  http.MethodPost,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    issuanceActionData.Body,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request to the certifier server: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	// Parse and validate the certificate response
	parsedCert, err := actions.ParseCertificateResponse(actions.ParseCertificateResponseParams{
		Response:    res,
		Args:        args,
		IdentityKey: key.PublicKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate response: %w", err)
	}

	// Verify server nonce
	if err := w.verifyNonce(ctx, parsedCert.ServerNonce, issuanceActionData.CounterParty, originator); err != nil {
		return nil, fmt.Errorf("failed to verify server nonce: %w", err)
	}

	// Verify the certificate against our request
	if err := actions.VerifyCertificateIssuance(ctx, w, parsedCert, nonce, issuanceActionData, key.PublicKey, args.Certifier, originator); err != nil {
		return nil, fmt.Errorf("failed to verify certificate: %w", err)
	}

	// Test that we can decrypt the certificate fields
	if err := actions.TestCertificateDecryption(ctx, w, parsedCert.CertFields, issuanceActionData.MasterKeyring, issuanceActionData.CounterParty, args); err != nil {
		return nil, fmt.Errorf("failed to test certificate decryption: %w", err)
	}

	// Store the certificate
	if err := actions.StoreCertificate(ctx, actions.StoreCertificateParams{
		Storage:            w.storage,
		Auth:               auth,
		Certificate:        parsedCert.Certificate,
		Certifier:          parsedCert.ParsedCertifier,
		RevocationOutpoint: parsedCert.RevocationOutpoint,
		Signature:          parsedCert.ParsedSignature,
		IdentityKey:        key.PublicKey,
		CertTypeB64:        issuanceActionData.CertTypeB64,
		Fields:             issuanceActionData.Fields,
		MasterKeyring:      issuanceActionData.MasterKeyring,
	}); err != nil {
		return nil, fmt.Errorf("failed to store certificate: %w", err)
	}

	// Build and return SDK certificate
	var serialNumberArray sdk.SerialNumber
	copy(serialNumberArray[:], parsedCert.SerialNumber)

	return &sdk.Certificate{
		Type:               args.Type,
		SerialNumber:       serialNumberArray,
		Subject:            key.PublicKey,
		Certifier:          parsedCert.ParsedCertifier,
		RevocationOutpoint: parsedCert.RevocationOutpoint,
		Fields:             args.Fields,
		Signature:          parsedCert.ParsedSignature,
	}, nil
}

// verifyNonce validates a nonce received from a counterparty.
// A nonce in this context is a unique value used to prevent replay attacks and ensure message integrity.
// The expected format of the nonce is a base64-encoded string containing 48 bytes:
//   - The first 16 bytes are arbitrary data (the nonce value).
//   - The next 32 bytes are an HMAC (Hash-based Message Authentication Code) of the data, used for integrity verification.
//
// This function decodes the nonce, checks its length, splits it into data and HMAC, and verifies the HMAC
// using the provided counterparty and originator information.
func (w *Wallet) verifyNonce(ctx context.Context, nonce string, counterparty sdk.Counterparty, originator string) error {
	// Convert nonce from base64 string to byte array
	buffer, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return fmt.Errorf("failed to decode nonce: %w", err)
	}

	// Validate nonce length (should be 16 bytes data + 32 bytes HMAC = 48 bytes)
	if len(buffer) < walletcerts.TotalNonceSize {
		return fmt.Errorf("invalid nonce length: expected at least %d bytes, got %d", walletcerts.TotalNonceSize, len(buffer))
	}

	// Split the nonce buffer
	data := buffer[:walletcerts.NonceDataSize]
	hmacSlice := buffer[walletcerts.NonceDataSize:]

	// Convert hmac slice to [32]byte array
	if len(hmacSlice) != walletcerts.NonceHMACSize {
		return fmt.Errorf("invalid hmac length: expected 32 bytes, got %d", len(hmacSlice))
	}

	var hmacArray [32]byte
	copy(hmacArray[:], hmacSlice)

	keyID := walletcerts.BytesToUTF8(data)

	// Verify the HMAC
	verifyHMACResult, err := w.VerifyHMAC(ctx, sdk.VerifyHMACArgs{
		Data: data,
		HMAC: hmacArray,
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID: sdk.Protocol{
				SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty,
				Protocol:      "server hmac",
			},
			KeyID:        keyID,
			Counterparty: counterparty,
		},
	}, originator)
	if err != nil {
		return fmt.Errorf("failed to verify HMAC: %w", err)
	}

	if !verifyHMACResult.Valid {
		return errors.New("HMAC verification failed: invalid nonce")
	}

	return nil
}

// createNonce generates a nonce for authentication and replay protection.
func (w *Wallet) createNonce(ctx context.Context, certifier *ec.PublicKey, originator string) (string, error) {
	nonce, err := walletcerts.CreateNonce(ctx, w, w.randomizer, certifier, originator)
	if err != nil {
		return "", fmt.Errorf("failed to create nonce for wallet: %w", err)
	}

	return nonce, nil
}

func (w *Wallet) acquireDirectCertificate(ctx context.Context, args sdk.AcquireCertificateArgs, originator string) (*sdk.Certificate, error) {
	w.logger.DebugContext(ctx, "AcquireCertificateDirect call", slogx.String("originator", originator))

	// Validate input arguments
	if err := validate.ValidateAcquireDirectCertificateArgs(&args); err != nil {
		return nil, fmt.Errorf("invalid AcquireCertificateArgs: %w", err)
	}

	// Retrieve authentication info
	auth, err := w.storage.GetAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve auth identity: %w", err)
	}

	// Prepare public key request arguments
	pubKeyArgs := sdk.GetPublicKeyArgs{IdentityKey: true}
	if args.Privileged != nil && to.Value(args.Privileged) {
		pubKeyArgs.Privileged = true
	}
	if len(args.PrivilegedReason) > 0 {
		pubKeyArgs.PrivilegedReason = args.PrivilegedReason
	}

	// Fetch the identity public key
	key, err := w.GetPublicKey(ctx, pubKeyArgs, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch identity public key: %w", err)
	}

	// Convert signature to hex string
	rHex := fmt.Sprintf("%064x", args.Signature.R)
	sHex := fmt.Sprintf("%064x", args.Signature.S)
	sigHex := rHex + sHex

	// Parse fields into TableCertificateField slice
	fields, err := wdk.ParseToTableCertificateFieldSlice(*auth.UserID, args.Fields, args.KeyringForSubject)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate fields for user %d: %w", *auth.UserID, err)
	}

	verifier := args.Certifier.ToDERHex()
	if args.KeyringRevealer != nil && args.KeyringRevealer.Certifier && args.KeyringRevealer.PubKey != nil {
		verifier = args.KeyringRevealer.PubKey.ToDERHex()
	}

	// Insert certificate into storage
	_, err = w.storage.InsertCertificateAuth(ctx, &wdk.TableCertificateX{
		TableCertificate: wdk.TableCertificate{
			UserID:             to.Value(auth.UserID),
			Type:               primitives.Base64String(base64.StdEncoding.EncodeToString(args.Type[:])),
			SerialNumber:       primitives.Base64String(base64.StdEncoding.EncodeToString(args.SerialNumber[:])),
			Certifier:          primitives.PubKeyHex(args.Certifier.ToDERHex()),
			Subject:            primitives.PubKeyHex(key.PublicKey.ToDERHex()),
			RevocationOutpoint: primitives.OutpointString(args.RevocationOutpoint.String()),
			Signature:          primitives.HexString(sigHex),
			Verifier:           to.Ptr(primitives.PubKeyHex(verifier)),
		},
		Fields: fields,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to insert certificate for user %d: %w", *auth.UserID, err)
	}

	// Build SDK certificate to return
	cert := sdk.Certificate{
		Type:               args.Type,
		SerialNumber:       to.Value(args.SerialNumber),
		Subject:            key.PublicKey,
		Certifier:          args.Certifier,
		RevocationOutpoint: args.RevocationOutpoint,
		Fields:             args.Fields,
		Signature:          args.Signature,
	}

	return &cert, nil
}

// ListCertificates lists identity certificates belonging to the user, filtered by certifier(s) and type(s).
func (w *Wallet) ListCertificates(ctx context.Context, args sdk.ListCertificatesArgs, originator string) (*sdk.ListCertificatesResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-ListCertificates", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "ListCertificates call", slogx.String("originator", originator))

	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	certifiers, types := mapping.MapListCertificatesArgs(args)
	listCertificatesResult, err := w.storage.ListCertificates(ctx, wdk.ListCertificatesArgs{
		Certifiers: certifiers,
		Types:      types,
		Limit:      primitives.PositiveIntegerDefault10Max10000(to.Value(args.Limit)),
		Offset:     primitives.PositiveInteger(to.Value(args.Offset)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates with given list certificates args: %w", err)
	}

	certs := make([]sdk.CertificateResult, 0, len(listCertificatesResult.Certificates))
	for _, storedModel := range listCertificatesResult.Certificates {
		cert, err := storedModel.ToSDKCertificate()
		if err != nil {
			return nil, fmt.Errorf("failed to parse stored certification result to sdk certificate: %w", err)
		}

		res := sdk.CertificateResult{Certificate: cert}
		if !storedModel.Keyring.IsEmpty() {
			res.Keyring = storedModel.Keyring.ToMap()
		}

		if !storedModel.Verifier.IsEmpty() {
			res.Verifier = []byte(storedModel.Verifier)
		}

		certs = append(certs, res)
	}

	totalCertificates := uint64(listCertificatesResult.TotalCertificates)
	if totalCertificates > math.MaxUint32 {
		return nil, fmt.Errorf("total certificates too large: %d", listCertificatesResult.TotalCertificates)
	}

	return &sdk.ListCertificatesResult{TotalCertificates: uint32(totalCertificates), Certificates: certs}, nil
}

// ProveCertificate proves select fields of an identity certificate, as specified, when requested by a verifier.
func (w *Wallet) ProveCertificate(ctx context.Context, args sdk.ProveCertificateArgs, originator string) (*sdk.ProveCertificateResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-ProveCertificate", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "ProveCertificate call", slogx.String("originator", originator))

	// Validation arguments and originator
	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}
	if err = validate.ProveCertificateArgs(args); err != nil {
		return nil, fmt.Errorf("failed to validate sdk.ProveCertificateArgs: %w", err)
	}

	// Convert signature to hex string
	cert := args.Certificate
	rHex := fmt.Sprintf("%064x", cert.Signature.R)
	sHex := fmt.Sprintf("%064x", cert.Signature.S)
	sigHex := rHex + sHex

	serialNumber := base64.StdEncoding.EncodeToString(cert.SerialNumber[:])

	// Fetch certificate from storage
	listCertificatesResult, err := w.storage.ListCertificates(ctx, wdk.ListCertificatesArgs{
		ListCertificatesArgsPartial: wdk.ListCertificatesArgsPartial{
			SerialNumber:       to.Ptr(primitives.Base64String(serialNumber)),
			Subject:            to.Ptr(primitives.PubKeyHex(cert.Subject.ToDERHex())),
			RevocationOutpoint: to.Ptr(primitives.OutpointString(cert.RevocationOutpoint.String())),
			Signature:          to.Ptr(primitives.HexString(sigHex)),
		},
		Limit:  primitives.PositiveIntegerDefault10Max10000(1),
		Offset: primitives.PositiveInteger(0),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates with given list certificates args: %w", err)
	}

	if listCertificatesResult.HasNoCertificates() {
		return nil, fmt.Errorf("certificate was not found with given list certificates args")
	}

	first := listCertificatesResult.First()
	certifier, err := first.CertifierCounterparty()
	if err != nil {
		return nil, fmt.Errorf("failed to create certifier counterparty: %w", err)
	}

	// Prepare fields for encryption
	fieldsForEncryption, err := first.Fields.ToFieldsForEncryption()
	if err != nil {
		return nil, fmt.Errorf("failed to parse wdk.CertificateResult fields to certificate fields to encryption: %w", err)
	}

	certificateFieldsResult, err := certificates.CreateCertificateFields(ctx, w, certifier, fieldsForEncryption, to.Value(args.Privileged), args.PrivilegedReason)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate fields: %w", err)
	}

	certificateFields := certificateFieldsResult.CertificateFields
	masterKeyring := certificateFieldsResult.MasterKeyring
	verifier := sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: args.Verifier}
	serial := sdk.StringBase64(base64.StdEncoding.EncodeToString(args.Certificate.SerialNumber[:]))

	// Validate certificate field names
	fieldNames, err := mapping.MapToCertificateFieldNameUnder50BytesSlice(certificateFields)
	if err != nil {
		return nil, fmt.Errorf("failed to map certificate fields to field names: %w", err)
	}

	// Create keyring for verifier
	keyringForVerifier, err := certificates.CreateKeyringForVerifier(
		ctx,
		w,
		certifier,
		verifier,
		certificateFields,
		fieldNames,
		masterKeyring,
		serial,
		to.Value(args.Privileged),
		args.PrivilegedReason,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create keyring for verifier: %w", err)
	}

	keyring := make(map[string]string, len(keyringForVerifier))
	for name, value := range keyringForVerifier {
		keyring[to.String(name)] = to.String(value)
	}

	return &sdk.ProveCertificateResult{KeyringForVerifier: keyring}, nil
}

// RelinquishCertificate relinquishes an identity certificate, removing it from the wallet regardless of whether
// the revocation outpoint has become spent.
func (w *Wallet) RelinquishCertificate(ctx context.Context, args sdk.RelinquishCertificateArgs, originator string) (*sdk.RelinquishCertificateResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-RelinquishCertificate", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "RelinquishCertificate call", slogx.String("originator", originator))

	// Validate input arguments
	mapped, err := mapping.MapRelinquishRelinquishCertificateArgs(args)
	if err != nil {
		return nil, fmt.Errorf("failed to map sdk.RelinquishCertificateArgs to wdk.RelinquishCertificateArgs: %w", err)
	}

	relArgs := wdk.RelinquishCertificateArgs{
		Type:         mapped.Type,
		SerialNumber: mapped.SerialNumber,
		Certifier:    mapped.Certifier,
	}
	if err := validate.RelinquishCertificateArgs(&relArgs); err != nil {
		return nil, fmt.Errorf("invalid RelinquishCertificateArgs: %w", err)
	}

	if err := w.storage.RelinquishCertificate(ctx, relArgs); err != nil {
		return nil, fmt.Errorf("failed to relinquish certificate: %w", err)
	}

	return &sdk.RelinquishCertificateResult{Relinquished: true}, nil
}

// DiscoverByIdentityKey discovers identity certificates, issued to a given identity key by a trusted entity.
func (w *Wallet) DiscoverByIdentityKey(ctx context.Context, args sdk.DiscoverByIdentityKeyArgs, originator string) (*sdk.DiscoverCertificatesResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-DiscoverByIdentityKey", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	now := time.Now()
	w.logger.DebugContext(ctx, "DiscoverByIdentityKey call", slogx.String("originator", originator))

	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	if err = validate.DiscoverByIdentityKeyArgs(args); err != nil {
		return nil, fmt.Errorf("failed to validate sdk.DiscoverByIdentityKeyArgs: %w", err)
	}

	certifiers := w.getCertifiers(now)
	identityKey := args.IdentityKey.ToDERHex()

	params, err := w.buildDiscoverParams(
		cacheKey{
			Fn:          "discoverByIdentityKey",
			IdentityKey: identityKey,
			Certifiers:  certifiers,
		},
		identityQuery{
			IdentityKey: identityKey,
			Certifiers:  certifiers,
		},
	)
	if err != nil {
		return nil, err
	}

	return w.discoverCertificates(ctx, params, now)
}

// DiscoverByAttributes discovers identity certificates belonging to other users, where the documents contain specific attributes, issued by a trusted entity.
func (w *Wallet) DiscoverByAttributes(ctx context.Context, args sdk.DiscoverByAttributesArgs, originator string) (*sdk.DiscoverCertificatesResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-DiscoverByAttributes", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	now := time.Now()
	w.logger.DebugContext(ctx, "DiscoverByAttributes call", slogx.String("originator", originator))

	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	if err = validate.DiscoverByAttributesArgs(args); err != nil {
		return nil, fmt.Errorf("failed to validate sdk.DiscoverByAttributesArgs: %w", err)
	}

	certifiers := w.getCertifiers(now)

	// Normalize attributes for a stable cache key.
	attributesKey, err := utils.SortedJSONString(args.Attributes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate sorted JSON string for attributes: %w", err)
	}

	params, err := w.buildDiscoverParams(
		cacheKey{
			Fn:         "discoverByAttributes",
			Attributes: attributesKey,
			Certifiers: certifiers,
		},
		attributesQuery{
			Attributes: args.Attributes,
			Certifiers: certifiers,
		},
	)
	if err != nil {
		return nil, err
	}

	return w.discoverCertificates(ctx, params, now)
}

// IsAuthenticated checks the authentication status of the user.
func (w *Wallet) IsAuthenticated(ctx context.Context, _ any, originator string) (*sdk.AuthenticatedResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-IsAuthenticated", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "IsAuthenticated call", slogx.String("originator", originator))
	err = validate.Originator(originator)
	if err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}
	return &sdk.AuthenticatedResult{
		Authenticated: true,
	}, nil
}

// WaitForAuthentication continuously waits until the user is authenticated, returning the result once confirmed.
func (w *Wallet) WaitForAuthentication(ctx context.Context, _ any, originator string) (*sdk.AuthenticatedResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-WaitForAuthentication", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "WaitForAuthentication call", slogx.String("originator", originator))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	return &sdk.AuthenticatedResult{
		Authenticated: true,
	}, nil
}

// GetHeight retrieves the current height of the blockchain.
func (w *Wallet) GetHeight(ctx context.Context, _ any, originator string) (*sdk.GetHeightResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-GetHeight", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "GetHeight call", slogx.String("originator", originator))
	if w.services == nil {
		return nil, fmt.Errorf("services are not configured for this wallet")
	}

	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	currentHeight, err := w.services.CurrentHeight(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current height: %w", err)
	}

	return &sdk.GetHeightResult{
		Height: currentHeight,
	}, nil
}

// GetHeaderForHeight retrieves the block header of a block at a specified height.
func (w *Wallet) GetHeaderForHeight(ctx context.Context, args sdk.GetHeaderArgs, originator string) (*sdk.GetHeaderResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-GetHeaderForHeight", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	w.logger.DebugContext(ctx, "GetHeaderForHeight call", slogx.String("originator", originator), logging.Number("height", args.Height))
	if w.services == nil {
		return nil, fmt.Errorf("wallet services not configured: cannot retrieve block header")
	}

	if err = validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkResult, err := w.services.ChainHeaderByHeight(ctx, args.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to get header for height %d: %w", args.Height, err)
	}

	result, err := mapping.MapGetHeaderResults(&wdkResult.ChainBaseBlockHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to map get header results: %w", err)
	}

	return result, nil
}

// GetNetwork retrieves the Bitcoin network the client is using (mainnet or testnet).
func (w *Wallet) GetNetwork(ctx context.Context, _ any, originator string) (*sdk.GetNetworkResult, error) {
	w.logger.DebugContext(ctx, "GetNetwork call", slogx.String("originator", originator))
	err := validate.Originator(originator)
	if err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	return &sdk.GetNetworkResult{
		Network: sdk.Network(w.chain),
	}, nil
}

// GetVersion retrieves the current version string of the wallet.
func (w *Wallet) GetVersion(ctx context.Context, _ any, originator string) (*sdk.GetVersionResult, error) {
	w.logger.DebugContext(ctx, "GetVersion call", slogx.String("originator", originator))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	return &sdk.GetVersionResult{
		Version: defs.Version,
	}, nil
}

// Close closes the wallet and all the components underneath.
func (w *Wallet) Close() {
	w.logger.DebugContext(context.Background(), "Close call")
	w.cleanup()
}

// Destroy is an alias for Close, that is an equivalent for the typescript wallet.destroy() method.
func (w *Wallet) Destroy() {
	w.logger.DebugContext(context.Background(), "Destroy call")
	w.Close()
}

// discoverCertificates is a shared helper for DiscoverByIdentityKey and DiscoverByAttributes.
// It handles trust settings, caching, overlay queries, and result mapping.
func (w *Wallet) discoverCertificates(ctx context.Context, params discoverCertificatesParams, now time.Time) (*sdk.DiscoverCertificatesResult, error) {
	trustSettings := w.getTrustSettings(now, discoverCertificatesTTL)

	// Check cache
	cached, ok := w.overlayCache.Load(params.cacheKeyStr)
	entry, typeOk := cached.(*cacheEntry)
	if !ok || !typeOk || !entry.ExpiresAt.After(now) {
		// Cache miss or expired - query overlay
		lookupAnswer, err := w.lookupResolver.Query(ctx, &lookup.LookupQuestion{
			Service: "ls_identity",
			Query:   params.query,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to query lookupResolver: %w", err)
		}

		verifiableCertificates := mapping.MapLookupAnswerToVerifiableCertificates(ctx, w.logger, lookupAnswer)

		// Store in cache
		cached = &cacheEntry{
			Value:     verifiableCertificates,
			ExpiresAt: now.Add(discoverCertificatesTTL),
		}
		w.overlayCache.Store(params.cacheKeyStr, cached)
	}

	entry, typeOk = cached.(*cacheEntry)
	if !typeOk || entry.Value == nil {
		return &sdk.DiscoverCertificatesResult{
			TotalCertificates: 0,
			Certificates:      []sdk.IdentityCertificate{},
		}, nil
	}

	verifiableCerts, err := mapping.MapVerifiableCertificatesWithTrust(w.logger, trustSettings, entry.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to map verifiable certificates with trust settings: %w", err)
	}
	return verifiableCerts, nil
}

func (w *Wallet) getTrustSettings(now time.Time, ttl time.Duration) *wallet_settings_manager.TrustSettings {
	cached := w.trustSettingsCache.Load()
	if cached != nil && cached.ExpiresAt.After(now) {
		return cached.TrustSettings
	}

	trustSettings := w.settingsManager.Get().TrustSettings
	w.trustSettingsCache.Store(&wallet_settings_manager.TrustSettingsCache{
		ExpiresAt:     now.Add(ttl),
		TrustSettings: trustSettings,
	})

	return trustSettings
}

// buildDiscoverParams builds the cache key and query for certificate discovery methods.
func (w *Wallet) buildDiscoverParams(cacheKeyData cacheKey, queryData any) (discoverCertificatesParams, error) {
	keyBytes, err := json.Marshal(cacheKeyData)
	if err != nil {
		return discoverCertificatesParams{}, fmt.Errorf("failed to marshal cacheKey: %w", err)
	}

	query, err := json.Marshal(queryData)
	if err != nil {
		return discoverCertificatesParams{}, fmt.Errorf("failed to marshal overlay query: %w", err)
	}

	return discoverCertificatesParams{
		cacheKeyStr: string(keyBytes),
		query:       query,
	}, nil
}

// getCertifiers returns sorted certifier identity keys from trust settings.
func (w *Wallet) getCertifiers(now time.Time) []string {
	trustSettings := w.getTrustSettings(now, discoverCertificatesTTL)
	certifiers := make([]string, len(trustSettings.TrustedCertifiers))
	for i, c := range trustSettings.TrustedCertifiers {
		certifiers[i] = c.IdentityKey
	}
	sort.Strings(certifiers)
	return certifiers
}

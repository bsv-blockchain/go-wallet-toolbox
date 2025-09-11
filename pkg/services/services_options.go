package services

import (
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

// Options represents configurable options for the wallet services component.
type Options struct {
	RestyClientFactory           *httpx.RestyClientFactory
	RawTxMethodsModifier         func([]Named[RawTxFunc]) []Named[RawTxFunc]
	PostBEEFMethodsModifier      func([]Named[PostBEEFFunc]) []Named[PostBEEFFunc]
	MerklePathMethodsModifier    func([]Named[MerklePathFunc]) []Named[MerklePathFunc]
	FindChainTipHeaderModifier   func([]Named[FindChainTipHeaderFunc]) []Named[FindChainTipHeaderFunc]
	IsValidRootForHeightModifier func([]Named[IsValidRootForHeightServicesFunc]) []Named[IsValidRootForHeightServicesFunc]
	CurrentHeightModifier        func([]Named[CurrentHeightFunc]) []Named[CurrentHeightFunc]
	GetScriptHashHistoryModifier func([]Named[GetScriptHashHistoryFunc]) []Named[GetScriptHashHistoryFunc]
	HashToHeaderModifier         func([]Named[HashToHeaderFunc]) []Named[HashToHeaderFunc]
	ChainHeaderByHeightModifier  func([]Named[ChainHeaderByHeightFunc]) []Named[ChainHeaderByHeightFunc]
	GetStatusForTxIDsModifier    func([]Named[GetStatusForTxIDsFunc]) []Named[GetStatusForTxIDsFunc]
	GetUtxoStatusModifier        func([]Named[GetUtxoStatusFunc]) []Named[GetUtxoStatusFunc]
	IsUtxoModifier               func([]Named[IsUtxo]) []Named[IsUtxo]
	BsvExchangeRateModifier      func([]Named[BsvExchangeRateFunc]) []Named[BsvExchangeRateFunc]

	customImplementations []Named[Implementation]
}

// WithHttpClient sets the http client for the service.
func WithHttpClient(client *http.Client) func(*Options) {
	r := resty.NewWithClient(client)
	return WithRestyClient(r)
}

// WithRestyClient sets the resty client for the WalletServices.
func WithRestyClient(client *resty.Client) func(*Options) {
	if client == nil {
		panic("client cannot be nil")
	}
	return func(o *Options) {
		o.RestyClientFactory = httpx.NewRestyClientFactoryWithBase(client)
	}
}

// WithCustomImplementation adds a custom implementation for service functions to the Options.
// You don't need to provide all functions - only those you want to add your own implementation for.
func WithCustomImplementation(name string, servicesDef Implementation) func(*Options) {
	return func(o *Options) {
		o.customImplementations = append(o.customImplementations, Named[Implementation]{
			Name: name,
			Item: servicesDef,
		})
	}
}

// WithRawTxMethodsModifier is designed to modify the list of RawTxFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithRawTxMethodsModifier(modifier func([]Named[RawTxFunc]) []Named[RawTxFunc]) func(*Options) {
	return func(o *Options) {
		o.RawTxMethodsModifier = modifier
	}
}

// WithPostBEEFMethodsModifier is designed to modify the list of PostBEEFFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithPostBEEFMethodsModifier(modifier func([]Named[PostBEEFFunc]) []Named[PostBEEFFunc]) func(*Options) {
	return func(o *Options) {
		o.PostBEEFMethodsModifier = modifier
	}
}

// WithMerklePathMethodsModifier is designed to modify the list of MerklePathFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithMerklePathMethodsModifier(modifier func([]Named[MerklePathFunc]) []Named[MerklePathFunc]) func(*Options) {
	return func(o *Options) {
		o.MerklePathMethodsModifier = modifier
	}
}

// WithFindChainTipHeaderModifier is designed to modify the list of FindChainTipHeaderFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithFindChainTipHeaderModifier(modifier func([]Named[FindChainTipHeaderFunc]) []Named[FindChainTipHeaderFunc]) func(*Options) {
	return func(o *Options) {
		o.FindChainTipHeaderModifier = modifier
	}
}

// WithIsValidRootForHeightModifier is designed to modify the list of IsValidRootForHeightServicesFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithIsValidRootForHeightModifier(modifier func([]Named[IsValidRootForHeightServicesFunc]) []Named[IsValidRootForHeightServicesFunc]) func(*Options) {
	return func(o *Options) {
		o.IsValidRootForHeightModifier = modifier
	}
}

// WithCurrentHeightModifier is designed to modify the list of CurrentHeightFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithCurrentHeightModifier(modifier func([]Named[CurrentHeightFunc]) []Named[CurrentHeightFunc]) func(*Options) {
	return func(o *Options) {
		o.CurrentHeightModifier = modifier
	}
}

// WithGetScriptHashHistoryModifier is designed to modify the list of GetScriptHashHistoryFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithGetScriptHashHistoryModifier(modifier func([]Named[GetScriptHashHistoryFunc]) []Named[GetScriptHashHistoryFunc]) func(*Options) {
	return func(o *Options) {
		o.GetScriptHashHistoryModifier = modifier
	}
}

// WithHashToHeaderModifier is designed to modify the list of HashToHeaderFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithHashToHeaderModifier(modifier func([]Named[HashToHeaderFunc]) []Named[HashToHeaderFunc]) func(*Options) {
	return func(o *Options) {
		o.HashToHeaderModifier = modifier
	}
}

// WithChainHeaderByHeightModifier is designed to modify the list of ChainHeaderByHeightFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithChainHeaderByHeightModifier(modifier func([]Named[ChainHeaderByHeightFunc]) []Named[ChainHeaderByHeightFunc]) func(*Options) {
	return func(o *Options) {
		o.ChainHeaderByHeightModifier = modifier
	}
}

// WithGetStatusForTxIDsModifier is designed to modify the list of GetStatusForTxIDsFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithGetStatusForTxIDsModifier(modifier func([]Named[GetStatusForTxIDsFunc]) []Named[GetStatusForTxIDsFunc]) func(*Options) {
	return func(o *Options) {
		o.GetStatusForTxIDsModifier = modifier
	}
}

// WithGetUtxoStatusModifier is designed to modify the list of GetUtxoStatusFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithGetUtxoStatusModifier(modifier func([]Named[GetUtxoStatusFunc]) []Named[GetUtxoStatusFunc]) func(*Options) {
	return func(o *Options) {
		o.GetUtxoStatusModifier = modifier
	}
}

// WithIsUtxoModifier is designed to modify the list of IsUtxo implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithIsUtxoModifier(modifier func([]Named[IsUtxo]) []Named[IsUtxo]) func(*Options) {
	return func(o *Options) {
		o.IsUtxoModifier = modifier
	}
}

// WithBsvExchangeRateModifier is designed to modify the list of BsvExchangeRateFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithBsvExchangeRateModifier(modifier func([]Named[BsvExchangeRateFunc]) []Named[BsvExchangeRateFunc]) func(*Options) {
	return func(o *Options) {
		o.BsvExchangeRateModifier = modifier
	}
}

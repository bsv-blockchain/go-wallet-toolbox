package services

import (
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracksclient"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

// Options represents configurable options for the wallet services component.
type Options struct {
	RestyClientFactory *httpx.RestyClientFactory

	RawTxMethodsModifier         func([]Named[RawTxFunc]) []Named[RawTxFunc]
	PostBEEFMethodsModifier      func([]Named[PostBEEFFunc]) []Named[PostBEEFFunc]
	MerklePathMethodsModifier    func([]Named[MerklePathFunc]) []Named[MerklePathFunc]
	FindChainTipHeaderModifier   func([]Named[FindChainTipHeaderFunc]) []Named[FindChainTipHeaderFunc]
	IsValidRootForHeightModifier func([]Named[IsValidRootForHeightFunc]) []Named[IsValidRootForHeightFunc]
	CurrentHeightModifier        func([]Named[CurrentHeightFunc]) []Named[CurrentHeightFunc]
	GetScriptHashHistoryModifier func([]Named[GetScriptHashHistoryFunc]) []Named[GetScriptHashHistoryFunc]
	HashToHeaderModifier         func([]Named[HashToHeaderFunc]) []Named[HashToHeaderFunc]
	ChainHeaderByHeightModifier  func([]Named[ChainHeaderByHeightFunc]) []Named[ChainHeaderByHeightFunc]
	GetStatusForTxIDsModifier    func([]Named[GetStatusForTxIDsFunc]) []Named[GetStatusForTxIDsFunc]
	GetUtxoStatusModifier        func([]Named[GetUtxoStatusFunc]) []Named[GetUtxoStatusFunc]
	IsUtxoModifier               func([]Named[IsUtxo]) []Named[IsUtxo]
	BsvExchangeRateModifier      func([]Named[BsvExchangeRateFunc]) []Named[BsvExchangeRateFunc]

	customImplementations []Named[Implementation]

	chaintracksAdapter *chaintracksclient.Adapter
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

// WithFindChainTipHeaderMethodsModifier is designed to modify the list of FindChainTipHeaderFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithFindChainTipHeaderMethodsModifier(modifier func([]Named[FindChainTipHeaderFunc]) []Named[FindChainTipHeaderFunc]) func(*Options) {
	return func(o *Options) {
		o.FindChainTipHeaderModifier = modifier
	}
}

// WithIsValidRootForHeightMethodsModifier is designed to modify the list of IsValidRootForHeightFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithIsValidRootForHeightMethodsModifier(modifier func([]Named[IsValidRootForHeightFunc]) []Named[IsValidRootForHeightFunc]) func(*Options) {
	return func(o *Options) {
		o.IsValidRootForHeightModifier = modifier
	}
}

// WithCurrentHeightMethodsModifier is designed to modify the list of CurrentHeightFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithCurrentHeightMethodsModifier(modifier func([]Named[CurrentHeightFunc]) []Named[CurrentHeightFunc]) func(*Options) {
	return func(o *Options) {
		o.CurrentHeightModifier = modifier
	}
}

// WithGetScriptHashHistoryMethodsModifier is designed to modify the list of GetScriptHashHistoryFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithGetScriptHashHistoryMethodsModifier(modifier func([]Named[GetScriptHashHistoryFunc]) []Named[GetScriptHashHistoryFunc]) func(*Options) {
	return func(o *Options) {
		o.GetScriptHashHistoryModifier = modifier
	}
}

// WithHashToHeaderMethodsModifier is designed to modify the list of HashToHeaderFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithHashToHeaderMethodsModifier(modifier func([]Named[HashToHeaderFunc]) []Named[HashToHeaderFunc]) func(*Options) {
	return func(o *Options) {
		o.HashToHeaderModifier = modifier
	}
}

// WithChainHeaderByHeightMethodsModifier is designed to modify the list of ChainHeaderByHeightFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithChainHeaderByHeightMethodsModifier(modifier func([]Named[ChainHeaderByHeightFunc]) []Named[ChainHeaderByHeightFunc]) func(*Options) {
	return func(o *Options) {
		o.ChainHeaderByHeightModifier = modifier
	}
}

// WithGetStatusForTxIDsMethodsModifier is designed to modify the list of GetStatusForTxIDsFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithGetStatusForTxIDsMethodsModifier(modifier func([]Named[GetStatusForTxIDsFunc]) []Named[GetStatusForTxIDsFunc]) func(*Options) {
	return func(o *Options) {
		o.GetStatusForTxIDsModifier = modifier
	}
}

// WithGetUtxoStatusMethodsModifier is designed to modify the list of GetUtxoStatusFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithGetUtxoStatusMethodsModifier(modifier func([]Named[GetUtxoStatusFunc]) []Named[GetUtxoStatusFunc]) func(*Options) {
	return func(o *Options) {
		o.GetUtxoStatusModifier = modifier
	}
}

// WithIsUtxoMethodsModifier is designed to modify the list of IsUtxo implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithIsUtxoMethodsModifier(modifier func([]Named[IsUtxo]) []Named[IsUtxo]) func(*Options) {
	return func(o *Options) {
		o.IsUtxoModifier = modifier
	}
}

// WithBsvExchangeRateMethodsModifier is designed to modify the list of BsvExchangeRateFunc implementations.
// The modifier function takes the current list of implementations and returns a modified list.
// The current list is made of the implementations provided via WithCustomImplementation and the built-in implementations.
// This allows you to change the order of implementations, add new ones, or remove existing ones.
func WithBsvExchangeRateMethodsModifier(modifier func([]Named[BsvExchangeRateFunc]) []Named[BsvExchangeRateFunc]) func(*Options) {
	return func(o *Options) {
		o.BsvExchangeRateModifier = modifier
	}
}

// WithChaintracksAdapter allows injecting a pre-configured chaintracks adapter.
// This is primarily useful for testing, where a mock chaintracks implementation
// can be injected to avoid real network calls and control test scenarios.
// When provided, the adapter will be used instead of creating one from config.
func WithChaintracksAdapter(adapter *chaintracksclient.Adapter) func(*Options) {
	return func(o *Options) {
		o.chaintracksAdapter = adapter
	}
}

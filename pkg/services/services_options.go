package services

import (
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

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

func WithCustomImplementation(name string, servicesDef Implementation) func(*Options) {
	return func(o *Options) {
		o.customImplementations = append(o.customImplementations, Named[Implementation]{
			Name: name,
			Item: servicesDef,
		})
	}
}

func WithRawTxMethodsModifier(modifier func([]Named[RawTxFunc]) []Named[RawTxFunc]) func(*Options) {
	return func(o *Options) {
		o.RawTxMethodsModifier = modifier
	}
}

func WithPostBEEFMethodsModifier(modifier func([]Named[PostBEEFFunc]) []Named[PostBEEFFunc]) func(*Options) {
	return func(o *Options) {
		o.PostBEEFMethodsModifier = modifier
	}
}

func WithMerklePathMethodsModifier(modifier func([]Named[MerklePathFunc]) []Named[MerklePathFunc]) func(*Options) {
	return func(o *Options) {
		o.MerklePathMethodsModifier = modifier
	}
}

func WithFindChainTipHeaderModifier(modifier func([]Named[FindChainTipHeaderFunc]) []Named[FindChainTipHeaderFunc]) func(*Options) {
	return func(o *Options) {
		o.FindChainTipHeaderModifier = modifier
	}
}

func WithIsValidRootForHeightModifier(modifier func([]Named[IsValidRootForHeightServicesFunc]) []Named[IsValidRootForHeightServicesFunc]) func(*Options) {
	return func(o *Options) {
		o.IsValidRootForHeightModifier = modifier
	}
}

func WithCurrentHeightModifier(modifier func([]Named[CurrentHeightFunc]) []Named[CurrentHeightFunc]) func(*Options) {
	return func(o *Options) {
		o.CurrentHeightModifier = modifier
	}
}

func WithGetScriptHashHistoryModifier(modifier func([]Named[GetScriptHashHistoryFunc]) []Named[GetScriptHashHistoryFunc]) func(*Options) {
	return func(o *Options) {
		o.GetScriptHashHistoryModifier = modifier
	}
}

func WithHashToHeaderModifier(modifier func([]Named[HashToHeaderFunc]) []Named[HashToHeaderFunc]) func(*Options) {
	return func(o *Options) {
		o.HashToHeaderModifier = modifier
	}
}

func WithChainHeaderByHeightModifier(modifier func([]Named[ChainHeaderByHeightFunc]) []Named[ChainHeaderByHeightFunc]) func(*Options) {
	return func(o *Options) {
		o.ChainHeaderByHeightModifier = modifier
	}
}

func WithGetStatusForTxIDsModifier(modifier func([]Named[GetStatusForTxIDsFunc]) []Named[GetStatusForTxIDsFunc]) func(*Options) {
	return func(o *Options) {
		o.GetStatusForTxIDsModifier = modifier
	}
}

func WithGetUtxoStatusModifier(modifier func([]Named[GetUtxoStatusFunc]) []Named[GetUtxoStatusFunc]) func(*Options) {
	return func(o *Options) {
		o.GetUtxoStatusModifier = modifier
	}
}

func WithIsUtxoModifier(modifier func([]Named[IsUtxo]) []Named[IsUtxo]) func(*Options) {
	return func(o *Options) {
		o.IsUtxoModifier = modifier
	}
}

func WithBsvExchangeRateModifier(modifier func([]Named[BsvExchangeRateFunc]) []Named[BsvExchangeRateFunc]) func(*Options) {
	return func(o *Options) {
		o.BsvExchangeRateModifier = modifier
	}
}

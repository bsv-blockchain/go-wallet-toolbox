package entity

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type TxGetterFcn func(ctx context.Context, txID string) (rawTx []byte, merklePath *transaction.MerklePath, err error)

type GetBEEFOptions struct {
	StatusesToFilterOut []wdk.ProvenTxReqStatus
	TxGetterFcn         TxGetterFcn
	MergeToBEEF         *transaction.Beef
	KnownTxIDs          []string
	KnownTxIDsSet       map[string]struct{}
	TrustSelf           wallet.TrustSelf
	MinProofLevel       int
	// DirectSourcesOnly stops the build at the subject transactions' immediate
	// parents, merged as raw transactions without merkle proofs or deeper
	// ancestry. Sufficient for script verification and EF construction (both
	// only need each input's source output), and skips the dominant costs of a
	// full build: per-ancestor BUMP root validation and recursive DB walks.
	DirectSourcesOnly bool
}

type GetBEEFOption = func(*GetBEEFOptions)

func WithStatusesToFilterOut(statuses ...wdk.ProvenTxReqStatus) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		opts.StatusesToFilterOut = statuses
	}
}

func WithMergeToBEEF(beef *transaction.Beef) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		opts.MergeToBEEF = beef
	}
}

func WithTxGetterFcn(txGetterFcn TxGetterFcn) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		opts.TxGetterFcn = txGetterFcn
	}
}

func WithKnownTxIDs(knownTxIDs ...string) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		if opts.KnownTxIDsSet == nil {
			opts.KnownTxIDsSet = make(map[string]struct{})
		}
		for _, txID := range knownTxIDs {
			opts.KnownTxIDsSet[txID] = struct{}{}
		}
	}
}

func WithTrustSelf(trust wallet.TrustSelf) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		opts.TrustSelf = trust
	}
}

// WithDirectSourcesOnly builds only the subject transactions plus their
// immediate parents (raw, proof-less). See GetBEEFOptions.DirectSourcesOnly.
func WithDirectSourcesOnly() GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		opts.DirectSourcesOnly = true
	}
}

func WithMinProofLevel(level int) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		opts.MinProofLevel = level
	}
}

func (ko *GetBEEFOptions) IsKnownTxID(txID string) bool {
	if ko.KnownTxIDsSet == nil {
		return false
	}
	_, ok := ko.KnownTxIDsSet[txID]
	return ok
}

func (ko *GetBEEFOptions) TrustsSelfAsKnown() bool {
	return ko.TrustSelf == wallet.TrustSelfKnown
}

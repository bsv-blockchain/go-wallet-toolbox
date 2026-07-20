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
	ProvenTxReqIDs      []string
	ProvenTxReqIDsSet   map[string]struct{}
	TrustSelf           wallet.TrustSelf
	MinProofLevel       int
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

func WithProvenTxReqIDs(provenTxReqIDs ...string) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		if opts.ProvenTxReqIDsSet == nil {
			opts.ProvenTxReqIDsSet = make(map[string]struct{})
		}
		for _, txID := range provenTxReqIDs {
			opts.ProvenTxReqIDsSet[txID] = struct{}{}
		}
	}
}

func WithTrustSelf(trust wallet.TrustSelf) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		opts.TrustSelf = trust
	}
}

func WithMinProofLevel(level int) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		opts.MinProofLevel = level
	}
}

func (ko *GetBEEFOptions) IsProvenTxReqID(txID string) bool {
	if ko.ProvenTxReqIDsSet == nil {
		return false
	}
	_, ok := ko.ProvenTxReqIDsSet[txID]
	return ok
}

func (ko *GetBEEFOptions) TrustsSelfAsKnown() bool {
	return ko.TrustSelf == wallet.TrustSelfKnown
}

package entity

import (
	"context"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type TxGetterFcn func(ctx context.Context, txID string) (rawTx []byte, merklePath *transaction.MerklePath, err error)

type GetBEEFOptions struct {
	StatusesToFilterOut []wdk.ProvenTxReqStatus
	TxGetterFcn         TxGetterFcn
	KnownTxIDs          []string
}

type GetBEEFOption = func(*GetBEEFOptions)

func WithStatusesToFilterOut(statuses ...wdk.ProvenTxReqStatus) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		opts.StatusesToFilterOut = statuses
	}
}

func WithTxGetterFcn(txGetterFcn TxGetterFcn) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		opts.TxGetterFcn = txGetterFcn
	}
}

func WithKnownTxIDs(knownTxIDs ...string) GetBEEFOption {
	return func(opts *GetBEEFOptions) {
		opts.KnownTxIDs = knownTxIDs
	}
}

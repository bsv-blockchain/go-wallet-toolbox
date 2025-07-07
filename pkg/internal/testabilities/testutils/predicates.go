package testutils

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"

func ProvidedByYouCondition(p *wdk.StorageCreateTransactionSdkOutput) bool {
	return p.ProvidedBy == wdk.ProvidedByYou
}

func ProvidedByStorageCondition(p *wdk.StorageCreateTransactionSdkOutput) bool {
	return p.ProvidedBy == wdk.ProvidedByStorage
}

func CommissionOutputCondition(p *wdk.StorageCreateTransactionSdkOutput) bool {
	return p.Purpose == "storage-commission"
}

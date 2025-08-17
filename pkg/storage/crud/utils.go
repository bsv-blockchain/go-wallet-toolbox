package crud

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"

func mapStringsToProvenTxStatuses(values []string) []wdk.ProvenTxReqStatus {
	statuses := make([]wdk.ProvenTxReqStatus, len(values))
	for i, v := range values {
		statuses[i] = wdk.ProvenTxReqStatus(v)
	}
	return statuses
}

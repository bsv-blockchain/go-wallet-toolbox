package validate

import (
	broadcastError "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/seq"
)

func NotDelayedProcessActionResult(result *wdk.ProcessActionResult) *broadcastError.BroadcastingError {
	if len(result.NotDelayedResults) == 0 || len(result.SendWithResults) == 0 {
		return nil
	}

	allSend := seq.Every(seq.FromSlice(result.SendWithResults), func(it wdk.SendWithResult) bool {
		return it.Status == wdk.SendWithResultStatusUnproven
	})

	if allSend {
		return nil
	}

	return broadcastError.NewValidationBroadcastError(result)
}

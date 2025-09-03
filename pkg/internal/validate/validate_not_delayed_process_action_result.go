package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/seq"
)

func NotDelayedProcessActionResult(result *wdk.ProcessActionResult) error {
	if len(result.NotDelayedResults) == 0 || len(result.SendWithResults) == 0 {
		return nil
	}

	allSend := seq.Every(seq.FromSlice(result.SendWithResults), func(it wdk.SendWithResult) bool {
		return it.Status == wdk.SendWithResultStatusUnproven
	})

	if allSend {
		return nil
	}

	return fmt.Errorf("unexpected not delayed results when send with results are not all unproven")
}

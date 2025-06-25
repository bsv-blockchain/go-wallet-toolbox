package validate

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
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

	// TODO: add more context to the error
	//   and allow for making errors.Is on it
	return fmt.Errorf("undelayed result require review")
}

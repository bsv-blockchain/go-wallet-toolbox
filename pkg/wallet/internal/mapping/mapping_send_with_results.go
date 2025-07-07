package mapping

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

func MapSendWithResultsFromWDKToSDK(sendWithResults []wdk.SendWithResult) ([]sdk.SendWithResult, error) {
	result, err := slices.MapOrError(sendWithResults, mapSendWithResultWDKtoSDK)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare SendWithResult list: %w", err)
	}
	return result, nil
}

func mapSendWithResultWDKtoSDK(sendWithResult wdk.SendWithResult) (sdk.SendWithResult, error) {
	txID, err := chainhash.NewHashFromHex(sendWithResult.TxID.String())
	if err != nil {
		return sdk.SendWithResult{}, fmt.Errorf("cannot restore tx id from hex %s, %w", sendWithResult.TxID, err)
	}

	status, err := mapSendWithResultStatusWDKtoSDK(sendWithResult.Status)
	if err != nil {
		return sdk.SendWithResult{}, err
	}

	return sdk.SendWithResult{
		Txid:   to.Value(txID),
		Status: status,
	}, nil
}

func mapSendWithResultStatusWDKtoSDK(status wdk.SendWithResultStatus) (sdk.ActionResultStatus, error) {
	switch status {
	case wdk.SendWithResultStatusUnproven:
		return sdk.ActionResultStatusUnproven, nil
	case wdk.SendWithResultStatusSending:
		return sdk.ActionResultStatusSending, nil
	case wdk.SendWithResultStatusFailed:
		return sdk.ActionResultStatusFailed, nil
	default:
		return "", fmt.Errorf("unexpected sendWithResult status from storage")
	}
}

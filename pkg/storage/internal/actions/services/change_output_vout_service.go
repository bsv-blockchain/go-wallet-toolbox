package services

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// TODO:
// 1. Check duplicates no send change X
// 2. Check len(outpoints) == outputs X
// 3. Check unique index for: user_id, tx_id, vout
// 4. Add missing integration tests

type OutputsRepository interface {
	FindOutputsByOutpoints(ctx context.Context, userID int, outpoints wdk.OutPointSlice) ([]*entity.Output, error)
}

type ChangeOutputVoutService struct {
	repository OutputsRepository
}

type CreateChangeOutputVoutsParams struct {
	UserID    int
	Basket    string
	Outpoints wdk.OutPointSlice
	TxOutputs []*entity.NewOutput
}

func (c *ChangeOutputVoutService) CreateNoSendChangeOutputVouts(ctx context.Context, params CreateChangeOutputVoutsParams) ([]int, error) {
	if !params.Outpoints.AreUnique() {
		return nil, fmt.Errorf("no send change outpoins are not unique")
	}

	outputs, err := c.repository.FindOutputsByOutpoints(ctx, params.UserID, params.Outpoints)
	if err != nil {
		return nil, fmt.Errorf("failed to find outputs by outpoints: %w", err)
	}

	if len(params.Outpoints) != len(outputs) {
		return nil, fmt.Errorf("failed to validate outputs: the number of outputs (%d) doesn't match the number of outpoints (%d)", len(outputs), len(params.Outpoints))
	}

	for _, output := range outputs {
		if output == nil {
			return nil, fmt.Errorf("failed to validate outputs: db query result contain a nil output value")
		}

		if output.ProvidedBy != wdk.ProvidedByStorage.String() {
			return nil, fmt.Errorf("failed to validate outputs: 'provided by' field value doesn't match %s value - output ID %d", wdk.ProvidedByStorage.String(), output.ID)
		}

		if output.Purpose != wdk.ChangePurpose {
			return nil, fmt.Errorf("failed to validate outputs: 'purpose' field value doesn't match %s value - output ID %d", wdk.ProvidedByStorage.String(), output.ID)
		}

		if output.BasketName == nil {
			return nil, fmt.Errorf("failed to validate outputs: 'basket name' field value is set to nil - output ID %d ", output.ID)
		}

		if *output.BasketName != params.Basket {
			return nil, fmt.Errorf("failed to validate outputs: 'basket name' field value doesn't match %s value - output ID %d ", params.Basket, output.ID)
		}

		// if !output.Spendable { // TODO: Decide how spendable property should be validated.
		// 	return fmt.Errorf("failed to validate outputs: 'spendable' field value is false - output ID %d", output.ID)
		// }
	}

	var vouts []int
	for _, output := range params.TxOutputs {
		if output.IsChangeOutputVout() {
			vouts = append(vouts, int(output.Vout))
		}
	}
	return vouts, nil
}

func NewChangeOutputVoutService(r OutputsRepository) *ChangeOutputVoutService {
	if r == nil {
		panic("outputs repository can't be nil")
	}

	return &ChangeOutputVoutService{repository: r}
}

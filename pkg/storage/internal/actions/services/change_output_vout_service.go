package services

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type OutputsRepository interface {
	FindOutputsByOutpoints(ctx context.Context, userID int, outpoints ...wdk.OutPoint) ([]*entity.Output, error)
}

type ChangeOutputVoutService struct {
	repository OutputsRepository
}

func (c *ChangeOutputVoutService) CreateNoSendChangeOutputVouts(outputs ...*entity.NewOutput) []int {
	var vouts []int
	for _, output := range outputs {
		if output.IsChangeOutputVout() {
			vouts = append(vouts, int(output.Vout))
		}
	}
	return vouts
}

// TODO:
// 1. Check duplicates no send change X
// 2. Check len(outpoints) == outputs X
// 3. Check unique index for: user_id, tx_id, vout
// 4. Add missing integration tests

func (c *ChangeOutputVoutService) ValidateNoSendChange(ctx context.Context, userID int, basket string, outpoints OutPointsSlice) error {
	if !outpoints.AreUnique() {
		return fmt.Errorf("no send change outpoins are not unique")
	}

	outputs, err := c.repository.FindOutputsByOutpoints(ctx, userID, outpoints...)
	if err != nil {
		return fmt.Errorf("failed to find outputs by outpoints: %w", err)
	}

	if len(outpoints) != len(outputs) {
		return fmt.Errorf("failed to validate outputs: the number of outputs (%d) doesn't match the number of outpoints (%d)", len(outputs), len(outpoints))
	}

	for _, output := range outputs {
		if output == nil {
			return fmt.Errorf("failed to validate outputs: db query result contain a nil output value")
		}

		if output.ProvidedBy != wdk.ProvidedByStorage.String() {
			return fmt.Errorf("failed to validate outputs: 'provided by' field value doesn't match %s value - output ID %d", wdk.ProvidedByStorage.String(), output.ID)
		}

		if output.Purpose != wdk.ChangePurpose {
			return fmt.Errorf("failed to validate outputs: 'purpose' field value doesn't match %s value - output ID %d", wdk.ProvidedByStorage.String(), output.ID)
		}

		if !output.Spendable {
			return fmt.Errorf("failed to validate outputs: 'spendable' field value is false -  output ID %d", output.ID)
		}

		if *output.BasketName != basket {
			return fmt.Errorf("failed to validate outputs: 'basket name' field value doesn't match %s value - output ID %d ", basket, output.ID)
		}
	}

	return nil
}

func NewChangeOutputVoutService(r OutputsRepository) *ChangeOutputVoutService {
	if r == nil {
		panic("outputs repository can't be nil")
	}

	return &ChangeOutputVoutService{repository: r}
}

type OutPointsSlice []wdk.OutPoint

func (outpoints OutPointsSlice) AreUnique() bool {
	m := make(map[string]bool)
	for _, outpoint := range outpoints {
		k := fmt.Sprintf("%s_%d", outpoint.TxID, outpoint.Vout)
		if ok := m[k]; ok {
			return false
		}
		m[k] = true
	}

	return true
}

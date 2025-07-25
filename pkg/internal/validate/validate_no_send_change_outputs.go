package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

var noSendChangeOutputValidators = []noSendChangeOutputValidator{
	validateNotNil,
	validateProvidedByStorage,
	validatePurposeChange,
	validateBasketNameChange,
	validateSpendable,
}

func NoSendChangeOutputs(outputs []*entity.Output) error {
	for _, output := range outputs {
		for _, validate := range noSendChangeOutputValidators {
			if err := validate(output); err != nil {
				return fmt.Errorf("validate no send change output error: %w", err)
			}
		}
	}

	return nil
}

type noSendChangeOutputValidator func(o *entity.Output) error

func validateNotNil(o *entity.Output) error {
	if o == nil {
		return fmt.Errorf("output is nil")
	}
	return nil
}

func validateProvidedByStorage(o *entity.Output) error {
	if o.ProvidedBy != string(wdk.ProvidedByStorage) {
		return fmt.Errorf("provided by field value doesn't match %s value - output ID %d", wdk.ProvidedByStorage, o.ID)
	}
	return nil
}

func validatePurposeChange(o *entity.Output) error {
	if o.Purpose != wdk.ChangePurpose {
		return fmt.Errorf("purpose field value doesn't match %s value - output ID %d", wdk.ChangePurpose, o.ID)
	}
	return nil
}

func validateBasketNameChange(o *entity.Output) error {
	if o.BasketName == nil {
		return fmt.Errorf("basket name field value is set to nil - output ID %d", o.ID)
	}
	if *o.BasketName != wdk.BasketNameForChange {
		return fmt.Errorf("basket name field value doesn't match %s value - output ID %d", wdk.BasketNameForChange, o.ID)
	}
	return nil
}

func validateSpendable(o *entity.Output) error {
	if !o.Spendable {
		return fmt.Errorf("spendable field value is false - output ID %d", o.ID)
	}
	return nil
}

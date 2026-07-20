package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func NoSendChangeOutputs(outputs []*entity.Output) error {
	for _, output := range outputs {
		if output == nil {
			return fmt.Errorf("output is nil")
		}

		if output.ProvidedBy != string(wdk.ProvidedByStorage) {
			return fmt.Errorf("provided by field value doesn't match %s value - output ID %d", wdk.ProvidedByStorage, output.ID)
		}

		if output.Purpose != wdk.ChangePurpose {
			return fmt.Errorf("purpose field value doesn't match %s value - output ID %d", wdk.ChangePurpose, output.ID)
		}

	}

	return nil
}

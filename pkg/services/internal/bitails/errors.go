package bitails

import "fmt"

var (
	ErrAlreadyKnown  = fmt.Errorf("already in mempool")
	ErrMissingInputs = fmt.Errorf("missing inputs")
	ErrDoubleSpend   = fmt.Errorf("double spend")
)

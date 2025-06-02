package primitives

import (
	"fmt"
	"strings"

	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/go-softwarelab/common/pkg/types"
)

// OutpointString represents a transaction ID and output index pair.
// The TXID is given as a hex string followed by a period "." and then the output index is given as a decimal integer.
type OutpointString string

// Validate checks if the string is proper outpoint string and contains outpoint index after "."
func (s OutpointString) Validate() error {
	split := strings.Split(string(s), ".")

	if len(split) != 2 {
		return fmt.Errorf("txid as hexstring and numeric output index joined with '.'")
	}

	// check if after decimal point there is an outpoint index
	_, err := to.UInt64FromString(split[1])
	if err != nil {
		return fmt.Errorf("txid as hexstring and numeric output index joined with '.'")
	}

	return nil
}

// Get parses the OutpointString and returns the transaction ID, output index, and an error if parsing fails.
func (s OutpointString) Get() (txID string, vout uint32, err error) {
	split := strings.Split(string(s), ".")
	if len(split) != 2 {
		return "", 0, fmt.Errorf("invalid outpoint string format: %s", s)
	}

	txID = split[0]
	vout32, err := to.UInt32FromString(split[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid vout index: %w", err)
	}

	return txID, vout32, nil
}

// MustGet parses the OutpointString and returns the transaction ID and output index, panicking on invalid format.
func (s OutpointString) MustGet() (txID string, vout uint32) {
	txID, vout, err := s.Get()
	if err != nil {
		panic(fmt.Sprintf("invalid outpoint string format: %s, error: %v", s, err))
	}

	return txID, vout
}

// MustGetTxID extracts and returns the transaction ID part from the OutpointString.
// It panics if the format is invalid.
func (s OutpointString) MustGetTxID() string {
	txID, _, err := s.Get()
	if err != nil {
		panic(fmt.Sprintf("invalid outpoint string format: %s, error: %v", s, err))
	}

	return txID
}

// MustGetVout extracts and returns the vout index from the OutpointString, panicking if the format is invalid.
func (s OutpointString) MustGetVout() uint32 {
	_, vout, err := s.Get()
	if err != nil {
		panic(fmt.Sprintf("invalid outpoint string format: %s, error: %v", s, err))
	}

	return vout
}

// NewOutpointString creates an OutpointString by joining the txid and index with a period separator.
func NewOutpointString[TxID ~string, Vout types.Integer](txid TxID, vout Vout) OutpointString {
	return OutpointString(fmt.Sprintf("%s.%d", txid, must.ConvertToUInt32(vout)))
}

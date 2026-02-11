package wdk

import (
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
)

// ValidCreateActionInput represents the input for a transaction action
type ValidCreateActionInput struct {
	Outpoint              OutPoint                      `json:"outpoint,omitempty"`
	InputDescription      primitives.String5to2000Bytes `json:"inputDescription,omitempty"`
	SequenceNumber        primitives.PositiveInteger    `json:"sequenceNumber,omitempty"`
	UnlockingScript       *primitives.HexString         `json:"unlockingScript,omitempty"`
	UnlockingScriptLength *primitives.PositiveInteger   `json:"unlockingScriptLength,omitempty"`
}

// ScriptLength returns the length of the unlocking script in bytes.
func (i *ValidCreateActionInput) ScriptLength() (uint64, error) {
	if i.UnlockingScript != nil {
		lengthInBytes, err := to.UInt64(len(*i.UnlockingScript) / 2)
		if err != nil {
			return 0, fmt.Errorf("failed to convert unlockingScript length: %w", err)
		}
		return lengthInBytes, nil
	}
	if i.UnlockingScriptLength != nil {
		return uint64(*i.UnlockingScriptLength), nil
	}
	return 0, fmt.Errorf("unlockingScript and unlockingScriptLength are both nil")
}

// ValidCreateActionOutput represents the output for a transaction action
type ValidCreateActionOutput struct {
	LockingScript      primitives.HexString          `json:"lockingScript,omitempty"`
	Satoshis           primitives.SatoshiValue       `json:"satoshis,omitempty"`
	OutputDescription  primitives.String5to2000Bytes `json:"outputDescription,omitempty"`
	Basket             *primitives.StringUnder300    `json:"basket,omitempty"`
	CustomInstructions *string                       `json:"customInstructions,omitempty"`
	Tags               []primitives.StringUnder300   `json:"tags,omitempty"`
}

// ScriptLength returns the length of the locking script in bytes.
func (o *ValidCreateActionOutput) ScriptLength() (uint64, error) {
	lengthInBytes, err := to.UInt64(len(o.LockingScript) / 2)
	if err != nil {
		return 0, fmt.Errorf("failed to convert lockingScript length: %w", err)
	}
	return lengthInBytes, nil
}

// ValidCreateActionOptions represents options for createAction
type ValidCreateActionOptions struct {
	AcceptDelayedBroadcast *primitives.BooleanDefaultTrue  `json:"acceptDelayedBroadcast,omitempty"`
	ReturnTXIDOnly         *primitives.BooleanDefaultFalse `json:"returnTXIDOnly,omitempty"`
	NoSend                 *primitives.BooleanDefaultFalse `json:"noSend,omitempty"`
	SendWith               []primitives.TXIDHexString      `json:"sendWith"`
	SignAndProcess         *primitives.BooleanDefaultTrue  `json:"signAndProcess,omitempty"`
	TrustSelf              *sdk.TrustSelf                  `json:"trustSelf,omitempty"`
	KnownTxids             []primitives.TXIDHexString      `json:"knownTxids"`
	NoSendChange           []OutPoint                      `json:"noSendChange"`
	RandomizeOutputs       bool                            `json:"randomizeOutputs"`
}

// ValidCreateActionArgs represents the arguments for creating a transaction action
type ValidCreateActionArgs struct {
	Description                  primitives.String5to2000Bytes `json:"description,omitempty"`
	InputBEEF                    primitives.BEEF               `json:"inputBEEF,omitempty"`
	Inputs                       []ValidCreateActionInput      `json:"inputs"`
	Outputs                      []ValidCreateActionOutput     `json:"outputs"`
	LockTime                     uint32                        `json:"lockTime,omitempty"`
	Version                      uint32                        `json:"version,omitempty"`
	Labels                       []primitives.StringUnder300   `json:"labels"`
	IsSignAction                 bool                          `json:"isSignAction,omitempty"`
	RandomVals                   *[]int                        `json:"randomVals,omitempty"`
	IncludeAllSourceTransactions bool                          `json:"includeAllSourceTransactions,omitempty"`
	Reference                    string                        `json:"reference,omitempty"`

	Options ValidCreateActionOptions `json:"options"`

	// Below are args from ValidProcessActionArgs

	// IsSendWith is true if a batch of transactions is included for processing
	IsSendWith bool `json:"isSendWith,omitempty"`
	// IsNewTx is true if there is a new transaction (not no inputs and no outputs)
	IsNewTx bool `json:"isNewTx,omitempty"`
	// IsRemixChange is true if this is a request to remix change
	// When true, IsNewTx will also be true and IsSendWith must be false
	IsRemixChange bool `json:"isRemixChange,omitempty"`
	// IsNoSend is true if any new transaction should NOT be sent to the network
	IsNoSend bool `json:"isNoSend,omitempty"`
	// IsDelayed is true if options.AcceptDelayedBroadcast is true
	IsDelayed bool `json:"isDelayed,omitempty"`
}

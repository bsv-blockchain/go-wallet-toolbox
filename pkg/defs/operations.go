package defs

// Operation defines the type for various operations in the wallet system.
type Operation string

const (
	// BackgroundBroadcast represents a background broadcasting operation.
	BackgroundBroadcast Operation = "backgroundBroadcast"

	// ImmediateBroadcast represents an immediate broadcasting operation.
	ImmediateBroadcast Operation = "immediateBroadcast"

	// DelayedBroadcast represents a delayed broadcasting operation.
	DelayedBroadcast Operation = "delayedBroadcast"

	// CreateAction represents an action to create a new wallet action.
	CreateAction Operation = "createAction"

	// ProcessAction represents an action to process an existing wallet action.
	ProcessAction Operation = "processAction"
)

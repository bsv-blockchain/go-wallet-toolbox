package specops

// ListActionsSpecOpFailedActionsLabel indicates that listActions should return only actions with status 'failed'.
const ListActionsSpecOpFailedActionsLabel = "97d4eb1e49215e3374cc2c1939a7c43a55e95c7427bf2d45ed63e3b4e0c88153"

// IsListActionsSpecOp returns true if the provided label is a reserved listActions spec-op.
func IsListActionsSpecOp(label string) bool {
	return label == ListActionsSpecOpFailedActionsLabel
}

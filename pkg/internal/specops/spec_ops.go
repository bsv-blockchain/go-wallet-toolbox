package specops

// ListActionsSpecOpFailedActionsLabel indicates that listActions should return only actions with status 'failed'.
const ListActionsSpecOpFailedActionsLabel = "97d4eb1e49215e3374cc2c1939a7c43a55e95c7427bf2d45ed63e3b4e0c88153" // #nosec G101

// ListOutputsSpecOpWalletBalance triggers a balance query: sum of satoshis for spendable outputs in the 'default' basket.
const ListOutputsSpecOpWalletBalance = "893b7646de0e1c9f741bd6e9169b76a8847ae34adef7bef1e6a285371206d2e8" // #nosec G101

// IsListActionsSpecOp returns true if the provided label is a reserved listActions spec-op.
func IsListActionsSpecOp(label string) bool {
	return label == ListActionsSpecOpFailedActionsLabel
}

// IsWalletBalanceSpecOp returns true if the basket is the reserved wallet balance spec-op.
func IsWalletBalanceSpecOp(basket string) bool {
	return basket == ListOutputsSpecOpWalletBalance
}

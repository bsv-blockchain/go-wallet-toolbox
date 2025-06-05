package wdk

const (
	// BasketNameForChange is the name of the output basket that is used to store "change" outputs
	BasketNameForChange = "default"

	// StorageCommissionPurpose is the purpose-string used for tagging storage commission outputs
	StorageCommissionPurpose = "storage-commission"

	// ChangePurpose is the purpose-string used for tagging change outputs
	ChangePurpose = "change"

	// NumberOfDesiredUTXOsForChange is the number of desired UTXOs for the change output basket,
	// it influences the number of change outputs created during createAction
	NumberOfDesiredUTXOsForChange = 32

	// MinimumDesiredUTXOValueForChange is the minimum value of UTXOs in the change output basket,
	// it influences the number of change outputs created during createAction
	MinimumDesiredUTXOValueForChange = 1000
)

// NonChangeBasketConfiguration defines default configuration for non-change output baskets.
// NOTE: Those parameters are used only by funder.Funder for "change" output baskets - so they are not applicable for non-change baskets.
// That's why the values are set to 0.
// This constant is made only to describe this fact and to avoid confusion.
var NonChangeBasketConfiguration = struct {
	NumberOfDesiredUTXOs    int64
	MinimumDesiredUTXOValue uint64
}{
	NumberOfDesiredUTXOs:    0,
	MinimumDesiredUTXOValue: 0,
}

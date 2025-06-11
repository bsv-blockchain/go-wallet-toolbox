package wdk

// RelinquishOutputArgs represents arguments for relinquishing output from the wallet storage.
type RelinquishOutputArgs struct {
	// NOTE: Basket is not used in TS code. However, BRC100 documentation says:
	// "args.basket - The associated basket name where the output should be removed."
	// So in current (go) implementation we'll stick to the documentation.

	Basket string `json:"basket"` // The associated basket name where the output should be removed

	// NOTE: The name "Output" is (I think, by mistake) misleading, because it refers to outPOINT so {txID}.{outputIndex}
	Output string `json:"output"`
}

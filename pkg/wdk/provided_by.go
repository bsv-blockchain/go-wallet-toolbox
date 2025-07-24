package wdk

// ProvidedBy indicates who provided the output.
type ProvidedBy string

// All possible values for ProvidedBy.
const (
	ProvidedByYou           ProvidedBy = "you"
	ProvidedByStorage       ProvidedBy = "storage"
	ProvidedByYouAndStorage ProvidedBy = "you-and-storage"
)

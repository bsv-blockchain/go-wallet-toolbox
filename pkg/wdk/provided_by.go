package wdk

// ProvidedBy indicates who provided the output.
type ProvidedBy string

// String returns the underlying string value.
func (p ProvidedBy) String() string { return string(p) }

// All possible values for ProvidedBy.
const (
	ProvidedByYou           ProvidedBy = "you"
	ProvidedByStorage       ProvidedBy = "storage"
	ProvidedByYouAndStorage ProvidedBy = "you-and-storage"
)

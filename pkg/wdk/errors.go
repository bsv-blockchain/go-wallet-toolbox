package wdk

import "fmt"

// ErrNotFoundError represents an error indicating that a requested resource or item was not found.
var ErrNotFoundError = fmt.Errorf("not found")

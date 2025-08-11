package wdk

import "fmt"

// NotFoundError represents an error indicating that a requested resource or item was not found.
var NotFoundError = fmt.Errorf("not found")

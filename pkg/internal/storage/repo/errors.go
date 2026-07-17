package repo

import "errors"

// ErrStatusUpdateSkipped signals a status UPDATE matched zero rows — either the
// row is absent, the skip-list excluded it, or an expected-status precondition
// failed. Callers decide whether a skip is legitimate; it is never silent.
var ErrStatusUpdateSkipped = errors.New("status update skipped: no matching rows")

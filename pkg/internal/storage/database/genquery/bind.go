package genquery

// This file is written by hand: the generator only writes gen.go and the *.gen.go files.

import "gorm.io/gorm"

// BindTx returns a copy of q whose tables run on tx's connection: the copy DBTransaction hands
// to its callback. It keeps the models and parsed schemas q was built with, which makes it much
// cheaper than Use(tx) for a short-lived handle, such as one per database transaction.
//
// Like the copy DBTransaction makes, the tables keep q's statement context, so queries take
// their context from WithContext.
func (q *Query) BindTx(tx *gorm.DB) *Query {
	return q.clone(tx)
}

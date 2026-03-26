package testabilities

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
)

func New(t testing.TB) (given FunderFixture, then FunderAssertion, cleanup func()) {
	db, cleanup := dbfixtures.TestDatabase(t)
	given, then = NewWithDatabase(t, db)
	return given, then, cleanup
}

func NewWithDatabase(t testing.TB, db *database.Database) (given FunderFixture, then FunderAssertion) {
	given = newFixture(t, db)
	fixture := given.(*funderFixture)
	then = newFunderAssertion(t, fixture)
	return given, then
}

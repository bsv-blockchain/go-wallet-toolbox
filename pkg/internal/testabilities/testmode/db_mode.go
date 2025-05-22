/*
Package testmode provides functions to set special modes for tests,
allowing to use actual Postgres or SQLite file for testing, especially for development purposes.
Important: It should be used only in LOCAL tests.
Calls of SetPostgresMode and SetFileSQLiteMode should not be committed.
*/
package testmode

import (
	"os"
	"testing"
)

const (
	modeEnvVar = "TEST_DB_MODE"
	nameEnvVar = "TEST_DB_NAME"

	defaultPostgresDBName  = "postgres"
	fileDbConnectionString = "file:storage.test.sqlite"
)

// WithDBName sets the database name for the test.
func WithDBName(dbName string) func(t testing.TB) {
	return func(t testing.TB) {
		t.Setenv(nameEnvVar, dbName)
	}
}

// DevelopmentOnly_SetPostgresMode sets the test mode to use actual Postgres and sets the database name.
func DevelopmentOnly_SetPostgresMode(t testing.TB, opts ...func(t testing.TB)) {
	t.Setenv(modeEnvVar, "postgres")
	for _, opt := range opts {
		opt(t)
	}
}

// DevelopmentOnly_SetFileSQLiteMode sets the test mode to use SQLite file
func DevelopmentOnly_SetFileSQLiteMode(t testing.TB) {
	t.Setenv(modeEnvVar, "file")
}

type Mode interface {
	ModeName() string
}

func GetMode() Mode {
	if ok, mode := checkSQLiteFileMode(); ok {
		return &mode
	}
	if ok, mode := checkPostgresMode(); ok {
		return &mode
	}
	return nil
}

type PostgresMode struct {
	DBName   string
	User     string
	Password string
	Host     string
}

func (m *PostgresMode) ModeName() string {
	return "postgres"
}

func checkPostgresMode() (ok bool, mode PostgresMode) {
	if os.Getenv(modeEnvVar) != mode.ModeName() {
		return false, mode
	}
	mode.DBName = os.Getenv(nameEnvVar)
	if mode.DBName == "" {
		mode.DBName = defaultPostgresDBName
	}
	mode.Host = "localhost"
	mode.User = "postgres"
	mode.Password = "postgres"
	return true, mode
}

type SQLiteFileMode struct {
	ConnectionString string
}

func (m *SQLiteFileMode) ModeName() string {
	return "file"
}

func checkSQLiteFileMode() (ok bool, mode SQLiteFileMode) {
	if os.Getenv(modeEnvVar) != mode.ModeName() {
		return false, mode
	}
	mode.ConnectionString = fileDbConnectionString
	return true, mode
}

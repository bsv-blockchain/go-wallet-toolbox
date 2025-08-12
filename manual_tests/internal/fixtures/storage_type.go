package fixtures

type StorageType string

const (
	StorageTypeLocalSQLite    StorageType = "local SQLite"
	StorageTypeLocalPostgres  StorageType = "local Postgres"
	StorageTypeRemoteSQLite   StorageType = "remote SQLite"
	StorageTypeRemotePostgres StorageType = "remote Postgres"
)

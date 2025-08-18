package fixtures

type StorageType string

const (
	StorageTypeLocalSQLite    StorageType = "local SQLite"
	StorageTypeLocalPostgres  StorageType = "local Postgres [not supported yet]"
	StorageTypeRemoteSQLite   StorageType = "remote SQLite [not supported yet]"
	StorageTypeRemotePostgres StorageType = "remote Postgres [not supported yet]"
)

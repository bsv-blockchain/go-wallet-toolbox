package fixtures

type StorageType string

const (
	StorageTypeLocalSQLite    StorageType = "local SQLite"
	StorageTypeRemoteSQLite   StorageType = "remote SQLite"
	StorageTypeRemotePostgres StorageType = "remote Postgres [not supported yet]"
)

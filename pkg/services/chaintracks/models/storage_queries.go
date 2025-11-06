package models

type StorageQueries interface {
	Begin()
	Rollback() error
	Commit() error

	LiveHeaderExists(hash string) (bool, error)
	GetLiveHeaderByHash(hash string) (*LiveBlockHeader, error)
}

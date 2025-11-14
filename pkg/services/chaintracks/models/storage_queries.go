package models

// StorageQueries defines an interface for querying and manipulating live blockchain headers in storage.
// It includes transactional methods for beginning, committing, and rolling back storage operations.
// Methods allow checking for header existence, retrieving headers, setting chain tip status, and counting headers.
type StorageQueries interface {
	Begin()
	Rollback() error
	Commit() error

	LiveHeaderExists(hash string) (bool, error)
	GetLiveHeaderByHash(hash string) (*LiveBlockHeader, error)
	GetActiveTipLiveHeader() (*LiveBlockHeader, error)
	SetChainTipByID(id uint, isChainTip bool) error
	InsertNewLiveHeader(header *LiveBlockHeader) error
	CountLiveHeaders() (int64, error)
}

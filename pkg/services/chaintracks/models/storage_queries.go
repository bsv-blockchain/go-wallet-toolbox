package models

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

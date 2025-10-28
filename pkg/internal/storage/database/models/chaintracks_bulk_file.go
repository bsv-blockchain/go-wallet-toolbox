package models

type ChaintracksBulkFile struct {
	FileID        uint    `gorm:"column:fileId;primaryKey;autoIncrement"`
	Chain         string  `gorm:"column:chain;not null;index:idx_bulk_files_first_height_chain"`
	FileName      string  `gorm:"column:fileName;not null"`
	FirstHeight   uint    `gorm:"column:firstHeight;not null;index:idx_bulk_files_first_height_chain"`
	Count         uint    `gorm:"column:count;not null"`
	PrevHash      string  `gorm:"column:prevHash;size:64;not null"`      // hex-encoded
	LastHash      string  `gorm:"column:lastHash;size:64;not null"`      // hex-encoded
	PrevChainWork string  `gorm:"column:prevChainWork;size:64;not null"` // hex-encoded
	LastChainWork string  `gorm:"column:lastChainWork;size:64;not null"` // hex-encoded
	FileHash      string  `gorm:"column:fileHash;not null"`              // base64-encoded
	Validated     bool    `gorm:"column:validated;not null;default:false"`
	SourceURL     *string `gorm:"column:sourceUrl"` // nullable
	Data          []byte  `gorm:"column:data"`      // nullable; large binary blob (up to ~32MB per migration)
}

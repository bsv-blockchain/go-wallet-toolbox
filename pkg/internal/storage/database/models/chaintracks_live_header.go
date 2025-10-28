package models

type ChaintracksLiveHeader struct {
	HeaderID         uint                   `gorm:"column:headerId;primaryKey;autoIncrement"`
	PreviousHeaderID *uint                  `gorm:"column:previousHeaderId;index:idx_live_headers_prev_header_id"`
	PreviousHeader   *ChaintracksLiveHeader `gorm:"foreignKey:PreviousHeaderID;references:HeaderID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`

	PreviousHash []byte `gorm:"column:previousHash"` // nullable; 32 bytes
	Height       uint   `gorm:"column:height;not null;index:idx_live_headers_height"`
	IsActive     bool   `gorm:"column:isActive;not null;index:idx_live_headers_is_active;index:idx_live_headers_is_active_chain_tip"`
	IsChainTip   bool   `gorm:"column:isChainTip;not null;index:idx_live_headers_is_chain_tip;index:idx_live_headers_is_active_chain_tip"`
	Hash         []byte `gorm:"column:hash;not null;uniqueIndex:ux_live_headers_hash"` // 32 bytes
	ChainWork    []byte `gorm:"column:chainWork;not null"`                             // 32 bytes
	Version      uint   `gorm:"column:version;not null"`
	MerkleRoot   []byte `gorm:"column:merkleRoot;not null;index:idx_live_headers_merkle_root"` // 32 bytes
	Time         uint   `gorm:"column:time;not null"`
	Bits         uint   `gorm:"column:bits;not null"`
	Nonce        uint   `gorm:"column:nonce;not null"`
}

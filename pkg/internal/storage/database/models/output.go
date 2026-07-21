package models

type Output struct {
	Timestamps

	OutputID      uint   `gorm:"column:outputId;primaryKey;autoIncrement"`
	UserID        int    `gorm:"column:userId;index;uniqueIndex:idx_output_tx_vout_user"`
	TransactionID uint   `gorm:"column:transactionId;index;uniqueIndex:idx_output_tx_vout_user"`
	SpentBy       *uint  `gorm:"column:spentBy;index"`
	Vout          uint32 `gorm:"column:vout;index;uniqueIndex:idx_output_tx_vout_user"`
	Satoshis      int64  `gorm:"column:satoshis"`

	LockingScript      []byte  `gorm:"column:lockingScript"`
	CustomInstructions *string `gorm:"column:customInstructions;type:string"`

	DerivationPrefix *string `gorm:"column:derivationPrefix;type:varchar(200)"`
	DerivationSuffix *string `gorm:"column:derivationSuffix;type:varchar(200)"`

	BasketID *uint         `gorm:"column:basketId"`
	Basket   *OutputBasket `gorm:"foreignKey:BasketID;references:BasketID"`

	Spendable bool `gorm:"column:spendable;index"`
	Change    bool `gorm:"column:change"`

	Description string `gorm:"column:outputDescription;type:varchar(2048)"`
	ProvidedBy  string `gorm:"column:providedBy"`
	Purpose     string `gorm:"column:purpose"`
	Type        string `gorm:"column:type"`

	SenderIdentityKey *string `gorm:"column:senderIdentityKey"`

	Txid                *string `gorm:"column:txid"`
	SequenceNumber      *uint32 `gorm:"column:sequenceNumber"`
	SpendingDescription *string `gorm:"column:spendingDescription"`
	ScriptLength        *uint32 `gorm:"column:scriptLength"`
	ScriptOffset        *uint32 `gorm:"column:scriptOffset"`

	Transaction        *Transaction `gorm:"foreignKey:TransactionID;references:TransactionID"`
	SpentByTransaction *Transaction `gorm:"foreignKey:SpentBy;references:TransactionID"`

	// Tags is populated/persisted manually via bsv_output_tags_map (see repo.Outputs) rather than
	// through a GORM many2many association: GORM's automatic join-table column derivation always
	// strips explicit `column:` tags and re-derives snake_case names, which cannot represent this
	// table's required camelCase FK columns (outputId/outputTagId per target-schema.md).
	Tags []*OutputTag `gorm:"-"`
}

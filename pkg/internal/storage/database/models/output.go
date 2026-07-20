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

	Tags []*OutputTag `gorm:"many2many:output_tags_map;joinForeignKey:OutputID;joinReferences:OutputTagID"`
}

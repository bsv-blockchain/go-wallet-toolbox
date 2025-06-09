package wdk

import "time"

type TableOutputAttrs struct {
	BasketName *string
}

// TableOutput represents a service model based on the TS version.
type TableOutput struct {
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	OutputID      uint      `json:"outputId"`
	UserID        int       `json:"userId"`
	TransactionID uint      `json:"transactionId"`
	//BasketID           *int      `json:"basketId,omitempty"`
	Spendable          bool    `json:"spendable"`
	Change             bool    `json:"change"`
	OutputDescription  string  `json:"outputDescription"`
	Vout               uint32  `json:"vout"`
	Satoshis           int64   `json:"satoshis"`
	ProvidedBy         string  `json:"providedBy"`
	Purpose            string  `json:"purpose"`
	Type               string  `json:"type"`
	TxID               *string `json:"txid,omitempty"`
	DerivationPrefix   *string `json:"derivationPrefix,omitempty"`
	DerivationSuffix   *string `json:"derivationSuffix,omitempty"`
	CustomInstructions *string `json:"customInstructions,omitempty"`
	LockingScript      *string `json:"lockingScript,omitempty"`
	SenderIdentityKey  *string `json:"senderIdentityKey,omitempty"`

	//BasketID   *int    `json:"basketId,omitempty"`
	BasketName *string `json:"-"`
}

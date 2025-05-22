package models

import (
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"gorm.io/datatypes"
)

type ProvenTxReq struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	TxID string `gorm:"type:varchar(64);primaryKey"`

	Status   wdk.ProvenTxReqStatus `gorm:"default:unknown"`
	Attempts uint
	Notified bool

	RawTx     []byte
	InputBeef []byte

	History datatypes.JSONType[*HistoryModel]
}

func (p *ProvenTxReq) AddNote(when time.Time, what string, attrs map[string]any) {
	note := HistoryNote{
		When:  when,
		What:  what,
		Attrs: attrs,
	}

	history := p.History.Data()
	if history == nil {
		history = &HistoryModel{}
		p.History = datatypes.NewJSONType(history)
	}

	history.Notes = append(history.Notes, note)
}

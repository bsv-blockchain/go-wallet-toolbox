package history

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-viper/mapstructure/v2"
)

const (
	InternalizeActionHistoryNote = "internalizeAction"
	ProcessActionHistoryNote     = "processAction"
	AggregateResultsHistoryNote  = "aggregateResults"
	NotifyTxOfProofHistoryNote   = "notifyTxOfProof"

	GetMerklePathSuccess  = "getMerklePathSuccess"
	GetMerklePathNotFound = "getMerklePathNotFound"

	PostBeefSuccess = "postBeefSuccess"
	PostBeefError   = "postBeefError"

	ServiceFetchedWhileGettingBeef = "serviceFetchedWhileGettingBeef"

	ReorgInvalidatedProof = "reorgInvalidatedProof"
)

const (
	statusNowAttr   = "status_now"
	serviceNameAttr = "name"
)

type EventTypesSelector interface {
	InternalizeAction(userID int) Builder
	ProcessAction(userID int) Builder
	AggregateResults(result AggregatedBroadcastResult) Builder
	NotifyTxOfProof(transactionID uint) Builder

	GetMerklePathSuccess(serviceName string) Builder
	GetMerklePathNotFound(serviceName string) Builder

	PostBeefError(serviceName string, beef TxData, txIDs []string, msg string) Builder
	PostBeefSuccess(serviceName string, txIDs []string) Builder

	ServiceFetchedWhileGettingBeef(subjectTxID string) Builder

	ReorgInvalidatedProof(orhpanedBlockHash string) Builder
}

type AggregatedBroadcastResult struct {
	StatusNow         wdk.ProvenTxReqStatus          `mapstructure:"status_now"`
	AggStatus         wdk.AggregatedPostedTxIDStatus `mapstructure:"aggStatus"`
	SuccessCount      int                            `mapstructure:"successCount"`
	DoubleSpendCount  int                            `mapstructure:"doubleSpendCount"`
	StatusErrorCount  int                            `mapstructure:"statusErrorCount"`
	ServiceErrorCount int                            `mapstructure:"serviceErrorCount"`
}

type Builder interface {
	WithUser(userID int) Builder
	WithWhat(what string) Builder
	WithAttributesFromObj(obj any) Builder
	WithAttribute(key string, value any) Builder
	WithNewStatus(status string) Builder

	Note() *wdk.HistoryNote
	Entity(txID string) *entity.TxHistoryNote
}

func NewBuilder() EventTypesSelector {
	return &builder{
		event: wdk.HistoryNote{
			When: time.Now(),
		},
	}
}

func NewBuilderFromNote(note *wdk.HistoryNote) Builder {
	return &builder{
		event: *note,
	}
}

type builder struct {
	event wdk.HistoryNote
}

func (b *builder) InternalizeAction(userID int) Builder {
	return b.WithWhat(InternalizeActionHistoryNote).WithUser(userID)
}

func (b *builder) ProcessAction(userID int) Builder {
	return b.WithWhat(ProcessActionHistoryNote).WithUser(userID)
}

func (b *builder) AggregateResults(result AggregatedBroadcastResult) Builder {
	return b.WithWhat(AggregateResultsHistoryNote).WithAttributesFromObj(result)
}

func (b *builder) GetMerklePathSuccess(serviceName string) Builder {
	return b.withHttpAttributes(http.StatusOK).
		WithWhat(GetMerklePathSuccess).
		WithAttribute(serviceNameAttr, serviceName)
}

func (b *builder) ServiceFetchedWhileGettingBeef(subjectTxID string) Builder {
	return b.withHttpAttributes(http.StatusOK).
		WithWhat(ServiceFetchedWhileGettingBeef).
		WithAttribute("subject_txid", subjectTxID)
}

func (b *builder) GetMerklePathNotFound(serviceName string) Builder {
	return b.withHttpAttributes(http.StatusNotFound).
		WithWhat(GetMerklePathNotFound).
		WithAttribute(serviceNameAttr, serviceName)
}

func (b *builder) PostBeefError(serviceName string, beef TxData, txIDs []string, msg string) Builder {
	return b.WithWhat(PostBeefError).
		WithAttribute(serviceNameAttr, serviceName).
		WithAttribute("hex", beef.toHex()).
		WithAttribute("txids", strings.Join(txIDs, ",")).
		WithAttribute("message", msg)
}

func (b *builder) PostBeefSuccess(serviceName string, txIDs []string) Builder {
	return b.WithWhat(PostBeefSuccess).
		WithAttribute(serviceNameAttr, serviceName).
		WithAttribute("txids", strings.Join(txIDs, ","))
}

func (b *builder) ReorgInvalidatedProof(orhpanedBlockHash string) Builder {
	return b.WithWhat(ReorgInvalidatedProof).
		WithAttribute("orhpaned_block_hash", orhpanedBlockHash).
		WithNewStatus(string(wdk.ProvenTxStatusReorg))
}

func (b *builder) NotifyTxOfProof(transactionID uint) Builder {
	return b.WithWhat(NotifyTxOfProofHistoryNote).
		WithAttribute("transactionId", transactionID)
}

func (b *builder) WithWhat(what string) Builder {
	b.event.What = what
	return b
}

func (b *builder) WithUser(userID int) Builder {
	b.event.UserID = &userID
	return b
}

func (b *builder) WithAttributesFromObj(obj any) Builder {
	if b.event.Attributes == nil {
		b.event.Attributes = make(map[string]any)
	}

	var objMap map[string]any
	err := mapstructure.Decode(obj, &objMap)
	if err != nil {
		panic(fmt.Errorf("failed to decode object to map: %w", err))
	}

	for key, value := range objMap {
		b.event.Attributes[key] = value
	}

	return b
}

func (b *builder) WithAttribute(key string, value any) Builder {
	if b.event.Attributes == nil {
		b.event.Attributes = make(map[string]any)
	}
	b.event.Attributes[key] = value
	return b
}

func (b *builder) WithNewStatus(status string) Builder {
	return b.WithAttribute(statusNowAttr, status)
}

func (b *builder) Note() *wdk.HistoryNote {
	return &b.event
}

func (b *builder) Entity(txID string) *entity.TxHistoryNote {
	return &entity.TxHistoryNote{
		TxID:        txID,
		HistoryNote: b.event,
	}
}

func (b *builder) withHttpAttributes(status int) Builder {
	return b.WithAttribute("status", status).WithAttribute("statusText", http.StatusText(status))
}

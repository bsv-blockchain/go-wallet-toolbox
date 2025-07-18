package history

import (
	"encoding/hex"
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

	GetMerklePathSuccess  = "getMerklePathSuccess"
	GetMerklePathNotFound = "getMerklePathNotFound"

	PostBeefSuccess = "postBeefSuccess"
	PostBeefError   = "postBeefError"
)

const (
	statusNowAttr   = "status_now"
	serviceNameAttr = "name"
)

type EventTypesSelector interface {
	InternalizeAction(userID int) Builder
	ProcessAction(userID int) Builder
	AggregateResults(result AggregatedBroadcastResult) Builder

	GetMerklePathSuccess(serviceName string) Builder
	GetMerklePathNotFound(serviceName string) Builder

	PostBeefError(serviceName string, beef []byte, txIDs []string, msg string) Builder
	PostBeefSuccess(serviceName string, beef []byte, txIDs []string) Builder
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
	return &spec{
		event: wdk.HistoryNote{
			When: time.Now(),
		},
	}
}

type spec struct {
	event wdk.HistoryNote
}

func (s *spec) InternalizeAction(userID int) Builder {
	return s.WithWhat(InternalizeActionHistoryNote).WithUser(userID)
}

func (s *spec) ProcessAction(userID int) Builder {
	return s.WithWhat(ProcessActionHistoryNote).WithUser(userID)
}

func (s *spec) AggregateResults(result AggregatedBroadcastResult) Builder {
	return s.WithWhat(AggregateResultsHistoryNote).WithAttributesFromObj(result)
}

func (s *spec) GetMerklePathSuccess(serviceName string) Builder {
	return s.withHttpAttributes(http.StatusOK).
		WithWhat(GetMerklePathSuccess).
		WithAttribute(serviceNameAttr, serviceName)
}

func (s *spec) GetMerklePathNotFound(serviceName string) Builder {
	return s.withHttpAttributes(http.StatusNotFound).
		WithWhat(GetMerklePathNotFound).
		WithAttribute(serviceNameAttr, serviceName)
}

func (s *spec) postBeefBase(what, serviceName string, beef []byte, txIDs []string) Builder {
	return s.WithWhat(what).
		WithAttribute(serviceNameAttr, serviceName).
		WithAttribute("beef", hex.EncodeToString(beef)).
		WithAttribute("txids", strings.Join(txIDs, ","))
}

func (s *spec) PostBeefError(serviceName string, beef []byte, txIDs []string, msg string) Builder {
	return s.postBeefBase(PostBeefError, serviceName, beef, txIDs).WithAttribute("message", msg)
}

func (s *spec) PostBeefSuccess(serviceName string, beef []byte, txIDs []string) Builder {
	return s.postBeefBase(PostBeefSuccess, serviceName, beef, txIDs)
}

func (s *spec) WithWhat(what string) Builder {
	s.event.What = what
	return s
}

func (s *spec) WithUser(userID int) Builder {
	s.event.UserID = &userID
	return s
}

func (s *spec) WithAttributesFromObj(obj any) Builder {
	if s.event.Attributes == nil {
		s.event.Attributes = make(map[string]any)
	}

	var objMap map[string]any
	err := mapstructure.Decode(obj, &objMap)
	if err != nil {
		panic(fmt.Errorf("failed to decode object to map: %w", err))
	}

	for key, value := range objMap {
		s.event.Attributes[key] = value
	}

	return s
}

func (s *spec) WithAttribute(key string, value any) Builder {
	if s.event.Attributes == nil {
		s.event.Attributes = make(map[string]any)
	}
	s.event.Attributes[key] = value
	return s
}

func (s *spec) WithNewStatus(status string) Builder {
	return s.WithAttribute(statusNowAttr, status)
}

func (s *spec) Note() *wdk.HistoryNote {
	return &s.event
}

func (s *spec) Entity(txID string) *entity.TxHistoryNote {
	return &entity.TxHistoryNote{
		TxID:        txID,
		HistoryNote: s.event,
	}
}

func (s *spec) withHttpAttributes(status int) Builder {
	return s.WithAttribute("status", status).WithAttribute("statusText", http.StatusText(status))
}

package history

import (
	"encoding/hex"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-viper/mapstructure/v2"
	"net/http"
	"strings"
	"time"
)

const (
	InternalizeActionHistoryNote = "internalizeAction"
	ProcessActionHistoryNote     = "processAction"
	AggregateResultsHistoryNote  = "aggregateResults"

	GetMerklePathSuccess  = "getMerklePathSuccess"
	GetMerklePathNotFound = "getMerklePathNotFound"

	PostBeefSuccess = "postBeefSuccess"
	PostBeefWarning = "postBeefWarning"
	PostBeefError   = "postBeefError"
)

const (
	statusNowAttr = "status_now"
)

type EventTypesSelector interface {
	InternalizeAction(userID int) Spec
	ProcessAction(userID int) Spec
	AggregateResults(result AggregatedBroadcastResult) Spec

	GetMerklePathSuccess(serviceName string) Spec
	GetMerklePathNotFound(serviceName string) Spec

	PostBeefError(serviceName string, beef []byte, txIDs []string, msg string) Spec
	PostBeefSuccess(serviceName string, beef []byte, txIDs []string) Spec
}

type AggregatedBroadcastResult struct {
	StatusNow         wdk.ProvenTxReqStatus          `mapstructure:"statusNow"`
	AggStatus         wdk.AggregatedPostedTxIDStatus `mapstructure:"aggStatus"`
	SuccessCount      int                            `mapstructure:"successCount"`
	DoubleSpendCount  int                            `mapstructure:"doubleSpendCount"`
	StatusErrorCount  int                            `mapstructure:"statusErrorCount"`
	ServiceErrorCount int                            `mapstructure:"serviceErrorCount"`
}

type Spec interface {
	WithUser(userID int) Spec
	WithName(name string) Spec
	WithAttributesFromObj(obj any) Spec
	WithAttribute(key string, value any) Spec
	WithNewStatus(status string) Spec

	Note() *wdk.HistoryNote
	Entity(txID string) *entity.TxHistoryNote
}

func New() EventTypesSelector {
	return &spec{
		event: wdk.HistoryNote{
			When: time.Now(),
		},
	}
}

type spec struct {
	event wdk.HistoryNote
}

func (s *spec) InternalizeAction(userID int) Spec {
	return s.WithName(InternalizeActionHistoryNote).WithUser(userID)
}

func (s *spec) ProcessAction(userID int) Spec {
	return s.WithName(ProcessActionHistoryNote).WithUser(userID)
}

func (s *spec) AggregateResults(result AggregatedBroadcastResult) Spec {
	return s.WithName(AggregateResultsHistoryNote).WithAttributesFromObj(result)
}

func (s *spec) GetMerklePathSuccess(serviceName string) Spec {
	return s.withHttpAttributes(http.StatusOK).
		WithName(GetMerklePathSuccess).
		WithAttribute("name", serviceName)
}

func (s *spec) GetMerklePathNotFound(serviceName string) Spec {
	return s.withHttpAttributes(http.StatusNotFound).
		WithName(GetMerklePathNotFound).
		WithAttribute("name", serviceName)
}

func (s *spec) postBeefBase(what, serviceName string, beef []byte, txIDs []string) Spec {
	return s.WithName(PostBeefSuccess).
		WithAttribute("name", serviceName).
		WithAttribute("beef", hex.EncodeToString(beef)).
		WithAttribute("txids", strings.Join(txIDs, ","))
}

func (s *spec) PostBeefError(serviceName string, beef []byte, txIDs []string, msg string) Spec {
	return s.postBeefBase(PostBeefError, serviceName, beef, txIDs).WithAttribute("message", msg)
}

func (s *spec) PostBeefSuccess(serviceName string, beef []byte, txIDs []string) Spec {
	return s.postBeefBase(PostBeefSuccess, serviceName, beef, txIDs)
}

func (s *spec) WithName(name string) Spec {
	s.event.What = name
	return s
}

func (s *spec) WithUser(userID int) Spec {
	s.event.UserID = &userID
	return s
}

func (s *spec) WithAttributesFromObj(obj any) Spec {
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

func (s *spec) WithAttribute(key string, value any) Spec {
	if s.event.Attributes == nil {
		s.event.Attributes = make(map[string]any)
	}
	s.event.Attributes[key] = value
	return s
}

func (s *spec) WithNewStatus(status string) Spec {
	return s.WithAttribute(statusNowAttr, status)
}

func (s *spec) Note() *wdk.HistoryNote {
	return &s.event
}

func (s *spec) Entity(txID string) *entity.TxHistoryNote {
	return &entity.TxHistoryNote{
		TxID:    txID,
		Content: s.event,
	}
}

func (s *spec) withHttpAttributes(status int) Spec {
	return s.WithAttribute("status", status).WithAttribute("statusText", http.StatusText(status))
}

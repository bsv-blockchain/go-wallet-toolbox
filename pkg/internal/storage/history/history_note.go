package history

import (
	"encoding/hex"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-viper/mapstructure/v2"
	"io"
	"net/http"
	"strings"
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
	statusNowAttr = "status_now"
	whatAttr      = "what"
	txIDAttr      = "tx_id"
	userIDAttr    = "user_id"
	whenAttr      = "when"
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

	Entity(txID string) *entity.TxNote
	ToMap() map[string]any
	AsList() *PlainList
}

func NewNote() EventTypesSelector {
	return &spec{
		event: entity.TxNote{},
	}
}

func NewList(specs ...Spec) *PlainList {
	notes := make([]map[string]any, 0, len(specs))
	for _, s := range specs {
		notes = append(notes, s.ToMap())
	}
	return &PlainList{notes: notes}
}

type PlainList struct {
	notes []map[string]any
}

func (p *PlainList) List() []map[string]any {
	return p.notes
}

func (p *PlainList) PrettyPrint(writer io.Writer) error {
	for _, note := range p.notes {
		for k, v := range note {
			if _, err := writer.Write([]byte(fmt.Sprintf("%s: %v\n", k, v))); err != nil {
				return err
			}
		}
		if _, err := writer.Write([]byte("\n\n")); err != nil {
			return err
		}
	}
	return nil
}

type spec struct {
	event entity.TxNote
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

func (s *spec) PostBeefError(serviceName string, beef []byte, txIDs []string, msg string) Spec {
	return s.WithName(PostBeefError).
		WithAttribute("name", serviceName).
		WithAttribute("beef", hex.EncodeToString(beef)).
		WithAttribute("txids", strings.Join(txIDs, ",")).
		WithAttribute("message", msg)
}

func (s *spec) PostBeefSuccess(serviceName string, beef []byte, txIDs []string) Spec {
	return s.WithName(PostBeefSuccess).
		WithAttribute("name", serviceName).
		WithAttribute("beef", hex.EncodeToString(beef)).
		WithAttribute("txids", strings.Join(txIDs, ","))
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

func (s *spec) Entity(txID string) *entity.TxNote {
	s.event.TxID = txID
	return &s.event
}

func (s *spec) ToMap() map[string]any {
	all := make(map[string]any, len(s.event.Attributes)+4)
	all[whatAttr] = s.event.What
	all[txIDAttr] = s.event.TxID
	all[userIDAttr] = s.event.UserID
	all[whenAttr] = s.event.When

	return all
}

func (s *spec) AsList() *PlainList {
	return NewList(s)
}

func (s *spec) withHttpAttributes(status int) Spec {
	return s.WithAttribute("status", status).WithAttribute("statusText", http.StatusText(status))
}

package history

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"

const (
	InternalizeActionHistoryNote = "internalizeAction"
	ProcessActionHistoryNote     = "processAction"
	AggregateResultsHistoryNote  = "aggregateResults"
)

const (
	statusNowAttr = "status_now"
)

type EventTypesSelector interface {
	InternalizeAction() Spec
	ProcessAction() Spec
	AggregateResults() Spec
}

type Spec interface {
	WithUser(userID int) Spec
	WithName(name string) Spec
	WithAttributes(attrs map[string]any) Spec
	WithAttribute(key string, value any) Spec
	WithNewStatus(status string) Spec

	Entity(txID string) *entity.TxNote
}

func NewNote() EventTypesSelector {
	return &spec{
		event: entity.TxNote{},
	}
}

type spec struct {
	event entity.TxNote
}

func (s *spec) InternalizeAction() Spec {
	return s.WithName(InternalizeActionHistoryNote)
}

func (s *spec) ProcessAction() Spec {
	return s.WithName(ProcessActionHistoryNote)
}

func (s *spec) AggregateResults() Spec {
	return s.WithName(AggregateResultsHistoryNote)
}

func (s *spec) WithName(name string) Spec {
	s.event.What = name
	return s
}

func (s *spec) WithUser(userID int) Spec {
	s.event.UserID = &userID
	return s
}

func (s *spec) WithAttributes(attrs map[string]any) Spec {
	if s.event.Attributes == nil {
		s.event.Attributes = make(map[string]any)
	}
	for k, v := range attrs {
		s.event.Attributes[k] = v
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

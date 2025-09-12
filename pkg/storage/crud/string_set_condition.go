package crud

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
)

// StringSetCondition provides filtering capabilities using ComparableSet[string]
type StringSetCondition[Parent any] interface {
	ContainAny(values ...string) Parent
	ContainAll(values ...string) Parent
	Empty() Parent
}

type stringSetCondition[Parent any] struct {
	parent          Parent
	conditionSetter func(*entity.ComparableSet[string])
}

func (s *stringSetCondition[Parent]) ContainAny(values ...string) Parent {
	s.conditionSetter(&entity.ComparableSet[string]{ContainAny: values})
	return s.parent
}

func (s *stringSetCondition[Parent]) ContainAll(values ...string) Parent {
	s.conditionSetter(&entity.ComparableSet[string]{ContainAll: values})
	return s.parent
}

func (s *stringSetCondition[Parent]) Empty() Parent {
	s.conditionSetter(&entity.ComparableSet[string]{Empty: true})
	return s.parent
}

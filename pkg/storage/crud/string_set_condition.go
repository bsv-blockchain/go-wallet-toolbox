package crud

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"

type StringSetCondition[Parent any] interface {
	Empty() Parent
	ContainAll(values ...string) Parent
	ContainAny(values ...string) Parent
}

type stringSetCondition[Parent any] struct {
	parent          Parent
	conditionSetter func(spec *entity.Comparable[string])
}

func (s *stringSetCondition[Parent]) Empty() Parent {
	//TODO implement me
	panic("implement me")
}

func (s *stringSetCondition[Parent]) ContainAny(values ...string) Parent {
	s.conditionSetter(&entity.Comparable[string]{
		InValues: values,
		Cmp:      entity.In,
	})

	return s.parent
}

func (s *stringSetCondition[Parent]) ContainAll(values ...string) Parent {
	//TODO implement me
	panic("implement me")
}

package services

import (
	"context"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/servicequeue"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/slices"
)

func applyModifierIfExists[F any](modifier func([]Named[F]) []Named[F], predefined []Named[F]) []Named[F] {
	if modifier == nil {
		return predefined
	}
	return modifier(predefined)
}

func namedFuncsToServices[R any](namedFuncs []Named[func(context.Context) (R, error)]) []*servicequeue.Service[R] {
	return slices.Map(namedFuncs, func(it Named[func(context.Context) (R, error)]) *servicequeue.Service[R] {
		return servicequeue.NewService(it.Name, it.Item)
	})
}

func namedFuncsToServices1[A, R any](namedFuncs []Named[func(context.Context, A) (R, error)]) []*servicequeue.Service1[A, R] {
	return slices.Map(namedFuncs, func(it Named[func(context.Context, A) (R, error)]) *servicequeue.Service1[A, R] {
		return servicequeue.NewService1(it.Name, it.Item)
	})
}

func namedFuncsToServices2[A, B, R any](namedFuncs []Named[func(context.Context, A, B) (R, error)]) []*servicequeue.Service2[A, B, R] {
	return slices.Map(namedFuncs, func(it Named[func(context.Context, A, B) (R, error)]) *servicequeue.Service2[A, B, R] {
		return servicequeue.NewService2(it.Name, it.Item)
	})
}

func collectSingleMethodImplementations[F any](servicesDefinitions []Named[Implementation], selector func(it Implementation) F) []Named[F] {
	var funcs []Named[F]
	for _, it := range servicesDefinitions {
		theFunc := selector(it.Item)
		if is.Nil(theFunc) {
			continue
		}

		funcs = append(funcs, Named[F]{Name: it.Name, Item: theFunc})
	}
	return funcs
}

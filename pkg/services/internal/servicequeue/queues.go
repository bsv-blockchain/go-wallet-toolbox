package servicequeue

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"runtime/debug"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/go-softwarelab/common/pkg/types"
)

var ErrEmptyResult = fmt.Errorf("service returns an empty result")
var ErrNoServicesRegistered = fmt.Errorf("no services registered")

// Queue is a structure that holds a collection of services and abstracts away the details of calling them and error handling.
// Services are functions accepting a context and returning a result or an error.
// For more arguments see Queue1, Queue2, and Queue3.
type Queue[R any] struct {
	logger     *slog.Logger
	methodName string
	services   []*Service[R]
}

func NewQueue[R any](logger *slog.Logger, methodName string, services ...*Service[R]) Queue[R] {
	logger = logging.Child(logger, "services."+methodName)
	logIfNamesAreNotUnique(services, logger)

	return Queue[R]{
		logger:     logger,
		methodName: methodName,
		services:   services,
	}
}

// All calls all services in parallel and returns the slice of results of all services.
func (q *Queue[R]) All(ctx context.Context) ([]*NamedResult[R], error) {
	return processParallel(ctx, q.logger, q.services, func(ctxParallel context.Context, s *Service[R]) (R, error) {
		return s.service(ctxParallel)
	})
}

// OneByOne calls the services with provided context, one by one, until a successful result is obtained.
// Returns the first successful result or an error if all services fail.
func (q *Queue[R]) OneByOne(ctx context.Context) (R, error) {
	return processOneByOne(q.logger, q.services, func(s *Service[R]) (R, error) {
		return s.service(ctx)
	})
}

// GetNames returns the method name and a slice of service names from the Queue1 instance.
func (q *Queue[R]) GetNames() (methodName string, serviceNames []string) {
	methodName = q.methodName
	serviceNames = make([]string, len(q.services))
	for i, s := range q.services {
		serviceNames[i] = s.Name()
	}
	return
}

// Queue1 is a structure that holds a collection of services and abstracts away the details of calling them and error handling.
// Services are functions accepting a context and an argument and returning a result or an error.
// For different number of arguments see Queue, Queue2, and Queue3.
type Queue1[A, R any] struct {
	logger     *slog.Logger
	methodName string
	services   []*Service1[A, R]
}

func NewQueue1[A, R any](logger *slog.Logger, methodName string, services ...*Service1[A, R]) Queue1[A, R] {
	logger = logging.Child(logger, "services."+methodName)
	logIfNamesAreNotUnique(services, logger)

	return Queue1[A, R]{
		logger:     logger,
		methodName: methodName,
		services:   services,
	}
}

// All processes all services in parallel and returns the slice of results of all services.
func (q *Queue1[A, R]) All(ctx context.Context, a A) ([]*NamedResult[R], error) {
	return processParallel(ctx, q.logger, q.services, func(ctxParallel context.Context, s *Service1[A, R]) (R, error) {
		return s.service(ctxParallel, a)
	})
}

// OneByOne processes services one by one until a successful result is obtained.
// The context and argument is passed to each service.
// Returns the first successful result or an error if all services fail.
func (q *Queue1[A, R]) OneByOne(ctx context.Context, a A) (R, error) {
	return processOneByOne(q.logger, q.services, func(s *Service1[A, R]) (R, error) {
		return s.service(ctx, a)
	})
}

// GetNames returns the method name and a slice of service names from the Queue1 instance.
func (q *Queue1[A, R]) GetNames() (methodName string, serviceNames []string) {
	methodName = q.methodName
	serviceNames = make([]string, len(q.services))
	for i, s := range q.services {
		serviceNames[i] = s.Name()
	}
	return
}

// Queue2 is a structure that holds a collection of services and abstracts away the details of calling them and error handling.
// Services are functions accepting a context and two arguments and returning a result or an error.
// For different number of arguments see Queue, Queue1, and Queue3.
type Queue2[A, B, R any] struct {
	logger     *slog.Logger
	methodName string
	services   []*Service2[A, B, R]
}

func NewQueue2[A, B, R any](logger *slog.Logger, methodName string, services ...*Service2[A, B, R]) Queue2[A, B, R] {
	logger = logging.Child(logger, "services."+methodName)
	logIfNamesAreNotUnique(services, logger)

	return Queue2[A, B, R]{
		logger:     logger,
		methodName: methodName,
		services:   services,
	}
}

// All processes all services in parallel and returns the slice of results of all services.
func (q *Queue2[A, B, R]) All(ctx context.Context, a A, b B) ([]*NamedResult[R], error) {
	return processParallel(ctx, q.logger, q.services, func(ctxParallel context.Context, s *Service2[A, B, R]) (R, error) {
		return s.service(ctxParallel, a, b)
	})
}

// OneByOne processes services one by one until a successful result is obtained.
// The context and arguments are passed to each service.
// Returns the first successful result or an error if all services fail.
func (q *Queue2[A, B, R]) OneByOne(ctx context.Context, a A, b B) (R, error) {
	return processOneByOne(q.logger, q.services, func(s *Service2[A, B, R]) (R, error) {
		return s.service(ctx, a, b)
	})
}

// GetNames returns the method name and a slice of service names from the Queue1 instance.
func (q *Queue2[A, B, R]) GetNames() (methodName string, serviceNames []string) {
	methodName = q.methodName
	serviceNames = make([]string, len(q.services))
	for i, s := range q.services {
		serviceNames[i] = s.Name()
	}
	return
}

// Queue3 is a structure that holds a collection of services and abstracts away the details of calling them and error handling.
// Services are functions accepting a context and three arguments and returning a result or an error.
// For different number of arguments see Queue, Queue1, and Queue2.
type Queue3[A, B, C, R any] struct {
	logger     *slog.Logger
	methodName string
	services   []*Service3[A, B, C, R]
}

func NewQueue3[A, B, C, R any](logger *slog.Logger, methodName string, services ...*Service3[A, B, C, R]) Queue3[A, B, C, R] {
	logger = logging.Child(logger, "services."+methodName)
	logIfNamesAreNotUnique(services, logger)

	return Queue3[A, B, C, R]{
		logger:     logger,
		methodName: methodName,
		services:   services,
	}
}

// All processes all services in parallel and returns the slice of results of all services.
func (q *Queue3[A, B, C, R]) All(ctx context.Context, a A, b B, c C) ([]*NamedResult[R], error) {
	return processParallel(ctx, q.logger, q.services, func(ctxParallel context.Context, s *Service3[A, B, C, R]) (R, error) {
		return s.service(ctxParallel, a, b, c)
	})
}

// OneByOne processes services one by one until a successful result is obtained.
// The context and arguments are passed to each service.
// Returns the first successful result or an error if all services fail.
func (q *Queue3[A, B, C, R]) OneByOne(ctx context.Context, a A, b B, c C) (R, error) {
	return processOneByOne(q.logger, q.services, func(s *Service3[A, B, C, R]) (R, error) {
		return s.service(ctx, a, b, c)
	})
}

// GetNames returns the method name and a slice of service names from the Queue1 instance.
func (q *Queue3[A, B, C, R]) GetNames() (methodName string, serviceNames []string) {
	methodName = q.methodName
	serviceNames = make([]string, len(q.services))
	for i, s := range q.services {
		serviceNames[i] = s.Name()
	}
	return
}

type serv interface {
	Name() string
}

func processParallel[S serv, R any](ctx context.Context, logger *slog.Logger, services []S, callService func(context.Context, S) (R, error)) ([]*NamedResult[R], error) {
	if len(services) == 0 {
		return nil, ErrNoServicesRegistered
	}

	results := internal.MapParallel(ctx, seq.FromSlice(services), func(ctxParallel context.Context, s S) (result *NamedResult[R]) {
		defer func() {
			if r := recover(); r != nil {
				var err error
				var ok bool
				if err, ok = r.(error); !ok {
					err = fmt.Errorf("%v", r)
				}
				err = fmt.Errorf("service %s has paniced with: %w \n %s", s.Name(), err, debug.Stack())
				result = NewNamedResult(s.Name(), types.FailureResult[R](err))
			}
		}()
		result = NewNamedResult(s.Name(), types.ResultOf(callService(ctxParallel, s)))
		return
	})

	results = seq.Map(results, func(result *NamedResult[R]) *NamedResult[R] {
		if result.IsNotError() && is.Nil(result.MustGetValue()) {
			return NewNamedResult(result.Name(), types.FailureResult[R](ErrEmptyResult))
		}
		return result
	})

	results = seq.Each(results, func(result *NamedResult[R]) {
		if result.IsError() {
			logger.Warn("error when calling service",
				slog.String("service.name", result.Name()),
				logging.Error(result.GetError()),
			)
		}
	})

	return sortedResults[S, R](services, results), nil
}

// sortedResults sorts the results based on the initial order of services.
func sortedResults[S serv, R any](services []S, results iter.Seq[*NamedResult[R]]) []*NamedResult[R] {
	initialOrderLookup := make(map[string]int, len(services))
	for i, service := range services {
		initialOrderLookup[service.Name()] = i
	}
	sorted := seq.SortBy(results, func(r *NamedResult[R]) int {
		return initialOrderLookup[r.Name()]
	})
	return seq.Collect(sorted)
}

func processOneByOne[S serv, R any](logger *slog.Logger, services []S, callService func(S) (R, error)) (R, error) {
	if len(services) == 0 {
		return to.ZeroValue[R](), ErrNoServicesRegistered
	}

	results := seq.Map(seq.FromSlice(services), func(s S) (result *NamedResult[R]) {
		defer func() {
			if r := recover(); r != nil {
				var err error
				var ok bool
				if err, ok = r.(error); !ok {
					err = fmt.Errorf("%v", r)
				}
				err = fmt.Errorf("service %s has paniced with: %w \n %s", s.Name(), err, debug.Stack())
				result = NewNamedResult(s.Name(), types.FailureResult[R](err))
			}
		}()
		res, err := callService(s)
		result = NewNamedResult(s.Name(), types.ResultOf(res, err))
		return
	})

	results = takeUntilHaveResult[R](results)

	results = seq.Each(results, logErrorResult[R](logger))

	var err error
	for result := range results {
		if result.IsError() {
			err = errors.Join(err, fmt.Errorf("error from service %s: %w", result.Name(), result.GetError()))
			continue
		}
		return result.MustGetValue(), nil
	}

	return to.ZeroValue[R](), fmt.Errorf("all services failed: %w", err)
}

func logErrorResult[R any](logger *slog.Logger) func(serviceResult *NamedResult[R]) {
	return func(serviceResult *NamedResult[R]) {
		if serviceResult.IsError() {
			logger.Warn("error when calling service",
				slog.String("service.name", serviceResult.Name()),
				logging.Error(serviceResult.GetError()),
			)
		}
	}
}

func takeUntilHaveResult[R any](seq iter.Seq[*NamedResult[R]]) iter.Seq[*NamedResult[R]] {
	return func(yield func(*NamedResult[R]) bool) {
		for result := range seq {
			if result.IsError() {
				if !yield(result) {
					break
				}
				continue
			}
			if is.Nil(result.MustGetValue()) {
				nilResult := NewNamedResult(result.Name(), types.FailureResult[R](ErrEmptyResult))
				if !yield(nilResult) {
					break
				}
				continue
			}

			yield(result)
			break
		}
	}
}

func logIfNamesAreNotUnique[T serv](services []T, logger *slog.Logger) {
	seen := make(map[string]struct{}, len(services))
	for _, s := range services {
		name := s.Name()
		if _, exists := seen[name]; exists {
			logger.Warn("duplicate service name detected", slog.String("service.name", name))
		} else {
			seen[name] = struct{}{}
		}
	}
}

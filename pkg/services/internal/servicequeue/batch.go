package servicequeue

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/types"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

// BatchService is a named service that takes many items in one call and answers with one
// result per item, in the order of the items.
type BatchService[A, R any] struct {
	name    string
	service func(context.Context, []A) ([]R, error)
}

// NewBatchService creates a new BatchService instance with given name.
func NewBatchService[A, R any](name string, service func(context.Context, []A) ([]R, error)) *BatchService[A, R] {
	return &BatchService[A, R]{
		name:    name,
		service: service,
	}
}

// Name returns the name of the service.
func (s *BatchService[A, R]) Name() string {
	return s.name
}

// BatchQueue holds a collection of batch services. All calls each of them once with every item.
type BatchQueue[A, R any] struct {
	logger     *slog.Logger
	history    *CallHistory
	methodName string
	services   []*BatchService[A, R]
}

// NewBatchQueue creates a new BatchQueue. methodName is the method its calls are logged and
// recorded under, which may be shared with the per-item queue of the same method.
func NewBatchQueue[A, R any](logger *slog.Logger, history *CallHistory, methodName string, services ...*BatchService[A, R]) BatchQueue[A, R] {
	logger = logging.Child(logger, "services."+methodName)
	logIfNamesAreNotUnique(services, logger)

	return BatchQueue[A, R]{
		logger:     logger,
		history:    history,
		methodName: methodName,
		services:   services,
	}
}

// Len returns the number of services in the queue.
func (q *BatchQueue[A, R]) Len() int {
	return len(q.services)
}

// GetNames returns the method name and a slice of service names.
func (q *BatchQueue[A, R]) GetNames() (methodName string, serviceNames []string) {
	methodName = q.methodName
	serviceNames = make([]string, len(q.services))
	for i, s := range q.services {
		serviceNames[i] = s.Name()
	}
	return methodName, serviceNames
}

// All calls every service once, in parallel, with all items, and returns for each item the
// results of every service, in service order.
//
// Each call is handled as a call of a per-item queue is: a panic, an error, or a missing
// result becomes an error result (for every item of the call), and each call is logged and
// recorded in the call history once. A service that answers with a number of results other
// than the number of items gets an error result for every item.
func (q *BatchQueue[A, R]) All(ctx context.Context, items []A) ([][]*NamedResult[R], error) {
	calls, err := processParallel(ctx, q.logger, q.history, q.methodName, q.services, func(ctxParallel context.Context, s *BatchService[A, R]) ([]R, error) {
		results, err := s.service(ctxParallel, items)
		if err == nil && results != nil && len(results) != len(items) {
			return nil, fmt.Errorf("service %s returned %d results for %d items", s.Name(), len(results), len(items))
		}
		return results, err
	})
	if err != nil {
		return nil, err
	}

	perItem := make([][]*NamedResult[R], len(items))
	for i := range items {
		perItem[i] = make([]*NamedResult[R], 0, len(calls))
		for _, call := range calls {
			perItem[i] = append(perItem[i], itemResult(call, i))
		}
	}
	return perItem, nil
}

// itemResult is the result for the i-th item of a batch call.
func itemResult[R any](call *NamedResult[[]R], i int) *NamedResult[R] {
	if call.IsError() {
		return NewNamedResult(call.Name(), types.FailureResult[R](call.GetError()))
	}
	value := call.MustGetValue()[i]
	if is.Nil(value) {
		return NewNamedResult(call.Name(), types.FailureResult[R](ErrEmptyResult))
	}
	return NewNamedResult(call.Name(), types.SuccessResult(value))
}

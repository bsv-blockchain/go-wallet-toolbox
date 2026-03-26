package servicequeue_test

import (
	"context"
	"errors"
	"testing"

	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/types"
	"github.com/stretchr/testify/assert"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/servicequeue"
)

const (
	secondArgument = "test"
	thirdArgument  = 1
	fourthArgument = true
)

var errFromPanic = errors.New("some panic occurred")

func TestQueueOneByOne(t *testing.T) {
	tests := map[string]struct {
		services         []TestService
		expectedResult   *TestServiceResult
		errorExpectation func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool
	}{
		"empty queue should return error": {
			services:         []TestService{},
			expectedResult:   nil,
			errorExpectation: assert.Error,
		},
		"single service returning success should return success": {
			services: []TestService{
				TestService{Name: "successful"}.Successful(),
			},
			expectedResult:   &TestServiceResult{200, "success"},
			errorExpectation: assert.NoError,
		},
		"single service returning error should return error": {
			services: []TestService{
				TestService{Name: "failing"}.Failing(),
			},
			expectedResult:   nil,
			errorExpectation: assert.Error,
		},
		"single service returning nil result should return error": {
			services: []TestService{
				TestService{Name: "nil-result"}.ReturningNilResult(),
			},
			expectedResult:   nil,
			errorExpectation: assert.Error,
		},
		"all services returning error should return error": {
			services: []TestService{
				TestService{Name: "error-service-1"}.Failing(),
				TestService{Name: "error-service-2"}.Failing(),
				TestService{Name: "error-service-3"}.Failing(),
			},
			expectedResult:   nil,
			errorExpectation: assert.Error,
		},
		"all services returning nil result should return error": {
			services: []TestService{
				TestService{Name: "nil-service-1"}.ReturningNilResult(),
				TestService{Name: "nil-service-2"}.ReturningNilResult(),
				TestService{Name: "nil-service-3"}.ReturningNilResult(),
			},
			expectedResult:   nil,
			errorExpectation: assert.Error,
		},
		"first service returning success should return that result and not call others": {
			services: []TestService{
				TestService{Name: "success-service"}.Successful(),
				TestService{Name: "should-not-be-called-1"}.ShouldNotBeCalled(),
				TestService{Name: "should-not-be-called-2"}.ShouldNotBeCalled(),
			},
			expectedResult:   &TestServiceResult{200, "success"},
			errorExpectation: assert.NoError,
		},
		"first service returning error but second returning success should return second result": {
			services: []TestService{
				TestService{Name: "error-service"}.Failing(),
				TestService{Name: "success-service"}.Successful(),
				TestService{Name: "should-not-be-called"}.ShouldNotBeCalled(),
			},
			expectedResult:   &TestServiceResult{200, "success"},
			errorExpectation: assert.NoError,
		},
		"first service returning nil but second returning success should return second result": {
			services: []TestService{
				TestService{Name: "nil-service"}.ReturningNilResult(),
				TestService{Name: "success-service"}.Successful(),
				TestService{Name: "should-not-be-called"}.ShouldNotBeCalled(),
			},
			expectedResult:   &TestServiceResult{200, "success"},
			errorExpectation: assert.NoError,
		},
		"first and second services returning error but third returning success should return third result": {
			services: []TestService{
				TestService{Name: "error-service-1"}.Failing(),
				TestService{Name: "error-service-2"}.Failing(),
				TestService{Name: "success-service"}.Successful(),
				TestService{Name: "should-not-be-called"}.ShouldNotBeCalled(),
			},
			expectedResult:   &TestServiceResult{200, "success"},
			errorExpectation: assert.NoError,
		},
		"first and second services returning nil but third returning success should return third result": {
			services: []TestService{
				TestService{Name: "nil-service-1"}.ReturningNilResult(),
				TestService{Name: "nil-service-2"}.ReturningNilResult(),
				TestService{Name: "success-service"}.Successful(),
				TestService{Name: "should-not-be-called"}.ShouldNotBeCalled(),
			},
			expectedResult:   &TestServiceResult{200, "success"},
			errorExpectation: assert.NoError,
		},
		"first service returning error, second returning nil, third returning success should return third result": {
			services: []TestService{
				TestService{Name: "error-service"}.Failing(),
				TestService{Name: "nil-service"}.ReturningNilResult(),
				TestService{Name: "success-service"}.Successful(),
				TestService{Name: "should-not-be-called"}.ShouldNotBeCalled(),
			},
			expectedResult:   &TestServiceResult{200, "success"},
			errorExpectation: assert.NoError,
		},
		"first service returning nil, second returning error, third returning success should return third result": {
			services: []TestService{
				TestService{Name: "nil-service"}.ReturningNilResult(),
				TestService{Name: "error-service"}.Failing(),
				TestService{Name: "success-service"}.Successful(),
				TestService{Name: "should-not-be-called"}.ShouldNotBeCalled(),
			},
			expectedResult:   &TestServiceResult{200, "success"},
			errorExpectation: assert.NoError,
		},
		"handle panic from service": {
			services: []TestService{
				TestService{Name: "panicking"}.Panicking(),
			},
			expectedResult: nil,
			errorExpectation: func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
				return assert.ErrorIs(t, err, errFromPanic, msgAndArgs...)
			},
		},
		"return result of second service if first service would panic": {
			services: []TestService{
				TestService{Name: "panicking"}.Panicking(),
				TestService{Name: "ok"}.Successful(),
			},
			expectedResult:   &TestServiceResult{200, "success"},
			errorExpectation: assert.NoError,
		},
	}
	for name, test := range tests {
		t.Run(name+": Queue", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service[*TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService(service.Name, service.Do)
			})

			// and:
			queue := servicequeue.NewQueue(
				logging.NewTestLogger(t),
				"Do",
				services...,
			)

			// when:
			r, err := queue.OneByOne(t.Context())

			// then:
			test.errorExpectation(t, err)
			assert.Equal(t, test.expectedResult, r)
		})

		t.Run(name+": Queue1", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service1[string, *TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService1(service.Name, service.Do1)
			})

			// and:
			queue := servicequeue.NewQueue1(
				logging.NewTestLogger(t),
				"Do1",
				services...,
			)

			// when:
			r, err := queue.OneByOne(t.Context(), secondArgument)

			// then:
			test.errorExpectation(t, err)
			assert.Equal(t, test.expectedResult, r)
		})

		t.Run(name+": Queue2", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service2[string, int, *TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService2(service.Name, service.Do2)
			})

			// and:
			queue := servicequeue.NewQueue2(
				logging.NewTestLogger(t),
				"Do2",
				services...,
			)

			// when:
			r, err := queue.OneByOne(t.Context(), secondArgument, thirdArgument)

			// then:
			test.errorExpectation(t, err)
			assert.Equal(t, test.expectedResult, r)
		})

		t.Run(name+": Queue3", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service3[string, int, bool, *TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService3(service.Name, service.Do3)
			})

			// and:
			queue := servicequeue.NewQueue3(
				logging.NewTestLogger(t),
				"Do3",
				services...,
			)

			// when:
			r, err := queue.OneByOne(t.Context(), secondArgument, thirdArgument, fourthArgument)

			// then:
			test.errorExpectation(t, err)
			assert.Equal(t, test.expectedResult, r)
		})
	}
}

func TestQueueParallel(t *testing.T) {
	testCases := map[string]struct {
		services         []TestService
		expectedResults  []*servicequeue.NamedResult[*TestServiceResult]
		errorExpectation func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool
	}{
		"empty queue should return error": {
			services:         []TestService{},
			expectedResults:  nil,
			errorExpectation: assert.Error,
		},
		"single service successful result": {
			services: []TestService{
				TestService{Name: "successful"}.Successful(),
			},
			expectedResults: []*servicequeue.NamedResult[*TestServiceResult]{
				servicequeue.NewNamedResult("successful", types.SuccessResult(&TestServiceResult{200, "success"})),
			},
			errorExpectation: assert.NoError,
		},
		"single service failure result": {
			services: []TestService{
				TestService{Name: "failing"}.Failing(),
			},
			expectedResults: []*servicequeue.NamedResult[*TestServiceResult]{
				servicequeue.NewNamedResult("failing", types.FailureResult[*TestServiceResult](errors.New("some error occurred"))),
			},
			errorExpectation: assert.NoError,
		},
		"single service nil result": {
			services: []TestService{
				TestService{Name: "nil-service"}.ReturningNilResult(),
			},
			expectedResults: []*servicequeue.NamedResult[*TestServiceResult]{
				servicequeue.NewNamedResult("nil-service", types.FailureResult[*TestServiceResult](servicequeue.ErrEmptyResult)),
			},
			errorExpectation: assert.NoError,
		},
		"multiple services with different results": {
			services: []TestService{
				TestService{Name: "successful"}.Successful(),
				TestService{Name: "failing"}.Failing(),
				TestService{Name: "ok"}.Successful(),
				TestService{Name: "nil-service"}.ReturningNilResult(),
				TestService{Name: "bad-thing-happened"}.Failing(),
				TestService{Name: "good-job"}.Successful(),
			},
			expectedResults: []*servicequeue.NamedResult[*TestServiceResult]{
				servicequeue.NewNamedResult("successful", types.SuccessResult(&TestServiceResult{200, "success"})),
				servicequeue.NewNamedResult("failing", types.FailureResult[*TestServiceResult](errors.New("some error occurred"))),
				servicequeue.NewNamedResult("ok", types.SuccessResult(&TestServiceResult{200, "success"})),
				servicequeue.NewNamedResult("nil-service", types.FailureResult[*TestServiceResult](servicequeue.ErrEmptyResult)),
				servicequeue.NewNamedResult("bad-thing-happened", types.FailureResult[*TestServiceResult](errors.New("some error occurred"))),
				servicequeue.NewNamedResult("good-job", types.SuccessResult(&TestServiceResult{200, "success"})),
			},
			errorExpectation: assert.NoError,
		},
	}
	for name, test := range testCases {
		t.Run(name+": Queue", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service[*TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService(service.Name, service.Do)
			})

			// and:
			queue := servicequeue.NewQueue(
				logging.NewTestLogger(t),
				"Do",
				services...,
			)

			// when:
			results, err := queue.All(t.Context())

			// then:
			test.errorExpectation(t, err)
			assert.Len(t, results, len(test.expectedResults))
			assert.ElementsMatch(t, test.expectedResults, results)
		})

		t.Run(name+": Queue1", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service1[string, *TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService1(service.Name, service.Do1)
			})

			// and:
			queue := servicequeue.NewQueue1(
				logging.NewTestLogger(t),
				"Do1",
				services...,
			)

			// when:
			results, err := queue.All(t.Context(), secondArgument)

			// then:
			test.errorExpectation(t, err)
			assert.Len(t, results, len(test.expectedResults))
			assert.ElementsMatch(t, test.expectedResults, results)
		})

		t.Run(name+": Queue2", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service2[string, int, *TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService2(service.Name, service.Do2)
			})

			// and:
			queue := servicequeue.NewQueue2(
				logging.NewTestLogger(t),
				"Do2",
				services...,
			)

			// when:
			results, err := queue.All(t.Context(), secondArgument, thirdArgument)

			// then:
			test.errorExpectation(t, err)
			assert.Len(t, results, len(test.expectedResults))
			assert.ElementsMatch(t, test.expectedResults, results)
		})

		t.Run(name+": Queue3", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service3[string, int, bool, *TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService3(service.Name, service.Do3)
			})

			// and:
			queue := servicequeue.NewQueue3(
				logging.NewTestLogger(t),
				"Do3",
				services...,
			)

			// when:
			results, err := queue.All(t.Context(), secondArgument, thirdArgument, fourthArgument)

			// then:
			test.errorExpectation(t, err)
			assert.Len(t, results, len(test.expectedResults))
			assert.ElementsMatch(t, test.expectedResults, results)
		})
	}

	panicTestCases := map[string]struct {
		services         []TestService
		expectedResults  []*servicequeue.NamedResult[*TestServiceResult]
		errorExpectation func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool
	}{
		"handle panic from service": {
			services: []TestService{
				TestService{Name: "panicking"}.Panicking(),
			},
			expectedResults: []*servicequeue.NamedResult[*TestServiceResult]{
				// Error is not exactly the same, because it contains more context about the source of the panic, but ErrorIs should be the same.
				servicequeue.NewNamedResult("panicking", types.FailureResult[*TestServiceResult](errFromPanic)),
			},
			errorExpectation: assert.NoError,
		},
		"handle panic of one service between multiple services": {
			services: []TestService{
				TestService{Name: "successful"}.Successful(),
				TestService{Name: "panicking"}.Panicking(),
				TestService{Name: "ok"}.Successful(),
			},
			expectedResults: []*servicequeue.NamedResult[*TestServiceResult]{
				servicequeue.NewNamedResult("successful", types.SuccessResult(&TestServiceResult{200, "success"})),
				servicequeue.NewNamedResult("panicking", types.FailureResult[*TestServiceResult](errFromPanic)),
				servicequeue.NewNamedResult("ok", types.SuccessResult(&TestServiceResult{200, "success"})),
			},
			errorExpectation: assert.NoError,
		},
	}
	for name, test := range panicTestCases {
		t.Run(name+": Queue", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service[*TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService(service.Name, service.Do)
			})

			// and:
			queue := servicequeue.NewQueue(
				logging.NewTestLogger(t),
				"Do",
				services...,
			)

			// when:
			results, err := queue.All(t.Context())

			// debug: show example of error from panicking service
			slices.ForEach(results, func(result *servicequeue.NamedResult[*TestServiceResult]) {
				if result.IsError() {
					t.Log(result.GetError())
				}
			})

			// then:
			test.errorExpectation(t, err)
			assert.Len(t, results, len(test.expectedResults))

			expectedResults := seq.FromSlice(test.expectedResults)
			slices.ForEach(results, func(result *servicequeue.NamedResult[*TestServiceResult]) {
				expected, err := seq.Find(expectedResults, func(r *servicequeue.NamedResult[*TestServiceResult]) bool {
					return result.Name() == r.Name()
				}).OrError(errors.New("got unexpected result from unknown service:" + result.Name()))
				if err != nil {
					t.Error(err)
					return
				}

				assert.Equal(t, expected.IsError(), result.IsError(), "got unexpected result type for service %s", result.Name())
				if result.IsError() {
					assert.ErrorIs(t, result.GetError(), expected.GetError(), "got unexpected error for service %s", result.Name())
				} else {
					assert.Equal(t, expected.MustGetValue(), result.MustGetValue(), "got unexpected result for service %s", result.Name())
				}
			})
		})

		t.Run(name+": Queue1", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service1[string, *TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService1(service.Name, service.Do1)
			})

			// and:
			queue := servicequeue.NewQueue1(
				logging.NewTestLogger(t),
				"Do1",
				services...,
			)

			// when:
			results, err := queue.All(t.Context(), secondArgument)

			// then:
			test.errorExpectation(t, err)
			assert.Len(t, results, len(test.expectedResults))

			expectedResults := seq.FromSlice(test.expectedResults)
			slices.ForEach(results, func(result *servicequeue.NamedResult[*TestServiceResult]) {
				expected, err := seq.Find(expectedResults, func(r *servicequeue.NamedResult[*TestServiceResult]) bool {
					return result.Name() == r.Name()
				}).OrError(errors.New("got unexpected result from unknown service:" + result.Name()))
				if err != nil {
					t.Error(err)
					return
				}

				assert.Equal(t, expected.IsError(), result.IsError(), "got unexpected result type for service %s", result.Name())
				if result.IsError() {
					assert.ErrorIs(t, result.GetError(), expected.GetError(), "got unexpected error for service %s", result.Name())
				} else {
					assert.Equal(t, expected.MustGetValue(), result.MustGetValue(), "got unexpected result for service %s", result.Name())
				}
			})
		})

		t.Run(name+": Queue2", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service2[string, int, *TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService2(service.Name, service.Do2)
			})

			// and:
			queue := servicequeue.NewQueue2(
				logging.NewTestLogger(t),
				"Do2",
				services...,
			)

			// when:
			results, err := queue.All(t.Context(), secondArgument, thirdArgument)

			// then:
			test.errorExpectation(t, err)
			assert.Len(t, results, len(test.expectedResults))

			expectedResults := seq.FromSlice(test.expectedResults)
			slices.ForEach(results, func(result *servicequeue.NamedResult[*TestServiceResult]) {
				expected, err := seq.Find(expectedResults, func(r *servicequeue.NamedResult[*TestServiceResult]) bool {
					return result.Name() == r.Name()
				}).OrError(errors.New("got unexpected result from unknown service:" + result.Name()))
				if err != nil {
					t.Error(err)
					return
				}

				assert.Equal(t, expected.IsError(), result.IsError(), "got unexpected result type for service %s", result.Name())
				if result.IsError() {
					assert.ErrorIs(t, result.GetError(), expected.GetError(), "got unexpected error for service %s", result.Name())
				} else {
					assert.Equal(t, expected.MustGetValue(), result.MustGetValue(), "got unexpected result for service %s", result.Name())
				}
			})
		})

		t.Run(name+": Queue3", func(t *testing.T) {
			// given:
			services := slices.Map(test.services, func(s TestService) *servicequeue.Service3[string, int, bool, *TestServiceResult] {
				service := s.NewTest(t)
				return servicequeue.NewService3(service.Name, service.Do3)
			})

			// and:
			queue := servicequeue.NewQueue3(
				logging.NewTestLogger(t),
				"Do3",
				services...,
			)

			// when:
			results, err := queue.All(t.Context(), secondArgument, thirdArgument, fourthArgument)

			// then:
			test.errorExpectation(t, err)
			assert.Len(t, results, len(test.expectedResults))

			expectedResults := seq.FromSlice(test.expectedResults)
			slices.ForEach(results, func(result *servicequeue.NamedResult[*TestServiceResult]) {
				expected, err := seq.Find(expectedResults, func(r *servicequeue.NamedResult[*TestServiceResult]) bool {
					return result.Name() == r.Name()
				}).OrError(errors.New("got unexpected result from unknown service:" + result.Name()))
				if err != nil {
					t.Error(err)
					return
				}

				assert.Equal(t, expected.IsError(), result.IsError(), "got unexpected result type for service %s", result.Name())
				if result.IsError() {
					assert.ErrorIs(t, result.GetError(), expected.GetError(), "got unexpected error for service %s", result.Name())
				} else {
					assert.Equal(t, expected.MustGetValue(), result.MustGetValue(), "got unexpected result for service %s", result.Name())
				}
			})
		})
	}
}

type TestServiceResult struct {
	StatusCode int
	Status     string
}

type TestService struct {
	Name         string
	t            testing.TB
	createResult func() (*TestServiceResult, error)
}

func (s TestService) Successful() TestService {
	s.createResult = func() (*TestServiceResult, error) {
		return &TestServiceResult{200, "success"}, nil
	}
	return s
}

func (s TestService) Failing() TestService {
	s.createResult = func() (*TestServiceResult, error) {
		return nil, errors.New("some error occurred")
	}
	return s
}

func (s TestService) ReturningNilResult() TestService {
	s.createResult = func() (*TestServiceResult, error) {
		return nil, nil
	}
	return s
}

func (s TestService) Panicking() TestService {
	s.createResult = func() (*TestServiceResult, error) {
		panic(errFromPanic)
	}
	return s
}

func (s TestService) ShouldNotBeCalled() TestService {
	s.createResult = func() (*TestServiceResult, error) {
		s.t.Fatalf("service %s shouldn't be called, but was.", s.Name)
		return nil, nil
	}
	return s
}

func (s TestService) NewTest(t testing.TB) *TestService {
	s.t = t
	return &s
}

func (s TestService) Do(ctx context.Context) (*TestServiceResult, error) {
	assert.NotNil(s.t, ctx, "expect to receive non-nil context as 1st argument")
	return s.createResult()
}

func (s TestService) Do1(ctx context.Context, str string) (*TestServiceResult, error) {
	assert.Equal(s.t, secondArgument, str, "expect to receive %#v as 2nd argument", secondArgument)
	return s.Do(ctx)
}

func (s TestService) Do2(ctx context.Context, str string, i int) (*TestServiceResult, error) {
	assert.Equal(s.t, thirdArgument, i, "expect to receive %#v as 3rd argument", thirdArgument)
	return s.Do1(ctx, str)
}

func (s TestService) Do3(ctx context.Context, str string, i int, boolean bool) (*TestServiceResult, error) {
	assert.Equal(s.t, fourthArgument, boolean, "expect to receive %#v as 4th argument", fourthArgument)
	return s.Do2(ctx, str, i)
}

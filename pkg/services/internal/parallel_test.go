package internal_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/go-softwarelab/common/pkg/types"
	"github.com/stretchr/testify/assert"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal"
)

func TestMapParallel(t *testing.T) {
	namesOfBigNumberTaskResults := seq.Map(seq.RangeTo(1000), to.StringFromInteger)
	bigNumberOfTasksResults := seq.Collect(seq.Map(namesOfBigNumberTaskResults, types.SuccessResult))

	testCases := map[string]struct {
		expectedResults []*types.Result[string]
		typeOfTask      func(result *types.Result[string]) *Task
	}{
		"handle empty sequence in input": {
			expectedResults: nil,
			typeOfTask:      FastTask,
		},
		"runs fast tasks in parallel": {
			expectedResults: []*types.Result[string]{
				types.SuccessResult("task-success"),
				types.SuccessResult("task-ok"),
				types.SuccessResult("task-done"),
			},
			typeOfTask: FastTask,
		},
		"runs tasks of various duration in parallel": {
			expectedResults: []*types.Result[string]{
				types.SuccessResult("task-success"),
				types.SuccessResult("task-ok"),
				types.SuccessResult("task-done"),
			},
			typeOfTask: SlowTask,
		},
		"handle single error in task": {
			expectedResults: []*types.Result[string]{
				types.SuccessResult("task-success"),
				types.FailureResult[string](errors.New("failure")),
				types.SuccessResult("task-done"),
			},
			typeOfTask: FastTask,
		},
		"handle all errors in tasks": {
			expectedResults: []*types.Result[string]{
				types.FailureResult[string](errors.New("failure")),
				types.FailureResult[string](errors.New("not-ok")),
				types.FailureResult[string](errors.New("bad thing happened")),
			},
			typeOfTask: FastTask,
		},
		"handle run big number of tasks short tasks in parallel": {
			expectedResults: bigNumberOfTasksResults,
			typeOfTask:      FastTask,
		},
		"handle run big number of tasks of various duration in parallel": {
			expectedResults: bigNumberOfTasksResults,
			typeOfTask:      SlowTask,
		},
	}
	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			// and:
			tasks := seq.Map(seq.FromSlice(test.expectedResults), test.typeOfTask)

			// when:
			taskResults := internal.MapParallel(t.Context(), tasks, func(_ context.Context, task *Task) *types.Result[string] {
				return task.Run(t)
			})

			results := seq.Collect(taskResults)

			// and:
			assert.ElementsMatch(t, results, test.expectedResults)
		})
	}

	t.Run("handle nil sequence", func(t *testing.T) {
		// when:
		taskResults := internal.MapParallel(t.Context(), nil, func(_ context.Context, task *Task) *types.Result[string] {
			return task.Run(t)
		})

		results := seq.Collect(taskResults)

		// and:
		assert.Empty(t, results, "result should be empty")
	})

	t.Run("handle finishing sequence earlier", func(t *testing.T) {
		// given:
		expectedResults := []*types.Result[string]{
			types.SuccessResult("task-success"),
			types.SuccessResult("task-ok"),
			types.SuccessResult("task-done"),
		}

		// and:
		tasks := seq.Map(seq.FromSlice(expectedResults), FastTask)

		// when:
		taskResults := internal.MapParallel(t.Context(), tasks, func(_ context.Context, task *Task) *types.Result[string] {
			return task.Run(t)
		})

		taskResults = seq.Take(taskResults, 1)

		results := seq.Collect(taskResults)

		// and:
		assert.Len(t, results, 1)
		assert.Contains(t, expectedResults, results[0], "result should be one of the expected results")
	})

	t.Run("handle context closed earlier", func(t *testing.T) {
		// given:
		expectedResults := bigNumberOfTasksResults

		// and:
		tasks := seq.Map(seq.FromSlice(expectedResults), SlowTask)

		// when: the context is closed
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		// and:
		taskResults := internal.MapParallel(ctx, tasks, func(_ context.Context, task *Task) *types.Result[string] {
			return task.Run(t)
		})

		results := seq.Collect(taskResults)

		// and:
		assert.Empty(t, results)
	})
}

type Task struct {
	name    string
	process func(tb testing.TB) types.Result[string]
}

func (t *Task) Run(tb testing.TB) *types.Result[string] {
	tb.Logf("Task %s started", t.name)
	res := t.process(tb)
	tb.Logf("Task %s finished", t.name)
	return &res
}

func FastTask(result *types.Result[string]) *Task {
	return &Task{
		name: result.OrElseGet(func() string {
			return result.GetError().Error()
		}),
		process: func(t testing.TB) types.Result[string] {
			return *result
		},
	}
}

func SlowTask(result *types.Result[string]) *Task {
	return &Task{
		name: result.OrElseGet(func() string {
			return result.GetError().Error()
		}),
		process: func(t testing.TB) types.Result[string] {
			delay := rand.IntN(500) //nolint:gosec // math/rand is sufficient for test timing delays
			time.Sleep(time.Duration(delay) * time.Millisecond)
			return *result
		},
	}
}

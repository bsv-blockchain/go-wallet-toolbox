package internal

import (
	"context"
	"iter"
	"sync"
)

const defaultChannelBuffer = 100

func MapParallel[E, R any](ctx context.Context, sequence iter.Seq[E], runner func(context.Context, E) R) iter.Seq[R] {
	if sequence == nil {
		return func(yield func(R) bool) {}
	}

	return func(yield func(R) bool) {
		wg := &sync.WaitGroup{}

		results := make(chan R, defaultChannelBuffer)

		childCtx, cancel := context.WithCancel(ctx)

	startGoRoutines:
		for v := range sequence {
			select {
			case <-childCtx.Done():
				break startGoRoutines
			default:
				wg.Add(1)
				go func(v E) {
					defer wg.Done()

					result := runner(childCtx, v)

					select {
					case <-childCtx.Done():
						return
					default:
						results <- result
					}
				}(v)
			}
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		for {
			select {
			case <-childCtx.Done():
				cancel()
				return
			case res, ok := <-results:
				if !ok {
					cancel()
					return
				}
				if !yield(res) {
					cancel()
					for range results {
						// drain the channel to avoid memory leaks
					}
					return
				}
			}
		}
	}
}

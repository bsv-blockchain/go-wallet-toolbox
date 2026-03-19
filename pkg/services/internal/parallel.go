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

		ctx, cancel := context.WithCancel(ctx)

	startGoRoutines:
		for v := range sequence {
			select {
			case <-ctx.Done():
				break startGoRoutines
			default:
				wg.Add(1)
				go func(v E) {
					defer wg.Done()

					result := runner(ctx, v)

					select {
					case <-ctx.Done():
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
			case <-ctx.Done():
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

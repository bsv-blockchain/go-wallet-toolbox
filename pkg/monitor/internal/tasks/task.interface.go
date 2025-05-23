package tasks

import "context"

type TaskInterface interface {
	Run(ctx context.Context) error
}

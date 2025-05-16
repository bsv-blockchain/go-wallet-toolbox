package tasks

import "log/slog"

type TaskCreator func(logger *slog.Logger) TaskInterface

type TaskInterface interface {
	Run()
}
